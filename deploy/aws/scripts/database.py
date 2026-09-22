"""Explicit three-database initialization and owner/runtime separation.

Uses an already-authenticated psql child with SQL on stdin; credentials never
appear in argv. Initialization refuses an existing database rather than changing
its owner/password or destroying state. Schema grants are a separate idempotent job.
"""

from common import require

DATABASES = {
    "agentclash": ("app_owner", "app_runtime"),
    "temporal": ("temporal_owner", "temporal_runtime"),
    "temporal_visibility": ("visibility_owner", "visibility_runtime"),
}


def literal(value):
    require(isinstance(value, str) and "\x00" not in value, "Invalid SQL value")
    return "'" + value.replace("'", "''") + "'"


def initialize_sql(passwords):
    require(
        set(passwords) == {role for pair in DATABASES.values() for role in pair},
        "Six database roles required",
    )
    sql = ["\\set ON_ERROR_STOP on", "SET standard_conforming_strings = on;"]
    # Not an upsert: rerunning requires operator reconciliation of partial state.
    for db, (owner, runtime) in DATABASES.items():
        for role in (owner, runtime):
            require(len(passwords[role]) >= 32, "Database password too short")
            sql.append(
                f"CREATE ROLE {role} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT CONNECTION LIMIT {4 if role == owner else 40} PASSWORD {literal(passwords[role])};"
            )
        sql += [
            f"GRANT {owner} TO CURRENT_USER;",
            f"CREATE DATABASE {db} OWNER {owner};",
            f"REVOKE ALL ON DATABASE {db} FROM PUBLIC;",
            f"GRANT CONNECT ON DATABASE {db} TO {runtime};",
            f"\\connect {db}",
            "REVOKE CREATE ON SCHEMA public FROM PUBLIC;",
            f"ALTER SCHEMA public OWNER TO {owner};",
            f"GRANT USAGE ON SCHEMA public TO {runtime};",
            f"ALTER DEFAULT PRIVILEGES FOR ROLE {owner} IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {runtime};",
            f"ALTER DEFAULT PRIVILEGES FOR ROLE {owner} IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO {runtime};",
            "\\connect postgres",
        ]
    return "\n".join(sql) + "\n"


def grants_sql(database):
    require(database in DATABASES, "Unknown database")
    owner, runtime = DATABASES[database]
    sql = f"""\\set ON_ERROR_STOP on
\\connect {database}
GRANT USAGE ON SCHEMA public TO {runtime};
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO {runtime};
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO {runtime};
ALTER DEFAULT PRIVILEGES FOR ROLE {owner} IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {runtime};
ALTER DEFAULT PRIVILEGES FOR ROLE {owner} IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO {runtime};
REVOKE INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM PUBLIC;
"""
    for table in ("schema_migrations", "schema_version", "schema_update_history"):
        sql += f"""DO $acl$ BEGIN
IF to_regclass('public.{table}') IS NOT NULL THEN
    REVOKE INSERT, UPDATE, DELETE ON public.{table} FROM {runtime};
END IF;
END $acl$;
"""
    return sql
