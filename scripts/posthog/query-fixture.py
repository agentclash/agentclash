"""Test-only relational execution of emitted HogQL in SQLite memory.

The adapter substitutes PostHog JSON access and ClickHouse scalar/aggregate
functions. This checks joins/windows/dedup on generated SQL, not PostHog parser
compatibility or production ingestion. No durable analytics store or network.
"""
import datetime
import json
import re
import sqlite3
import sys

payload = json.load(sys.stdin)
db = sqlite3.connect(":memory:")
def epoch(value, _zone="UTC"):
    return datetime.datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()
def week(value, _mode, _zone):
    dt = datetime.datetime.fromtimestamp(value, datetime.timezone.utc)
    return (dt - datetime.timedelta(days=dt.weekday())).date().isoformat()
class ArgMin:
    def __init__(self):
        self.value, self.at = None, float("inf")
    def step(self, value, at):
        if at < self.at:
            self.value, self.at = value, at
    def finalize(self):
        return self.value
db.create_function("toDateTime", 2, epoch)
db.create_function("toStartOfWeek", 3, week)
db.create_function("toInt", 1, lambda x: int(x) if x is not None else None)
db.create_function("toString", 1, lambda x: str(x) if x is not None else None)
db.create_function("notEmpty", 1, lambda x: bool(x))
db.create_function("match", 2, lambda value, pattern: bool(re.search(pattern, value or "")))
db.create_function("if", 3, lambda cond, yes, no: yes if cond else no)
db.create_aggregate("argMin", 2, ArgMin)
db.execute("CREATE TABLE events(event TEXT, distinct_id TEXT, timestamp REAL, properties TEXT, person TEXT)")
for event in payload["events"]:
    # PostHog JSON boolean strings are normalized by its toString; preserve that
    # representation in this adapter rather than SQLite's numeric JSON bool.
    props = {k: str(v).lower() if isinstance(v, bool) else v for k, v in event.get("properties", {}).items()}
    person = {k: str(v).lower() if isinstance(v, bool) else v for k, v in event.get("person", {}).items()}
    db.execute("INSERT INTO events VALUES(?,?,?,?,?)", (event["event"], event["distinct_id"], epoch(event["timestamp"]), json.dumps(props), json.dumps(person)))
output = {}
for spec in payload["insights"]:
    sql = spec["query"]["source"]["query"]
    sql = re.sub(r"pdi\.person\.properties\.([\w$]+)", lambda m: "json_extract(person, '$." + m[1] + "')", sql)
    sql = re.sub(r"(?<!person\.)properties\.([\w$]+)", lambda m: "json_extract(properties, '$." + m[1] + "')", sql)
    sql = sql.replace("INTERVAL 7 DAY", "(7 * 86400)")
    cursor = db.execute(sql)
    output[spec["name"]] = [dict(zip([column[0] for column in cursor.description], row)) for row in cursor.fetchall()]
json.dump(output, sys.stdout)
