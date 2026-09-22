"""Local PostgreSQL credentials through libpq environment, never process argv."""
import os
from urllib.parse import parse_qs, unquote, urlparse

def connection_env(dsn):
    url=urlparse(dsn)
    if url.scheme not in {'postgres','postgresql'} or url.hostname not in {'localhost','127.0.0.1','::1'}:
        raise ValueError('explicit local PostgreSQL URL required')
    query=parse_qs(url.query)
    if set(query)-{'sslmode'}:raise ValueError('unsupported connection URL parameters')
    env={k:v for k,v in os.environ.items() if not k.startswith('PG')}
    env.update(PGHOST=url.hostname,PGPORT=str(url.port or 5432),PGDATABASE=unquote(url.path.lstrip('/')),
               PGUSER=unquote(url.username or ''),PGPASSWORD=unquote(url.password or ''),
               PGSSLMODE=query.get('sslmode',['prefer'])[0],PGCONNECT_TIMEOUT='10',PGAPPNAME='vibe-phase1')
    return env
