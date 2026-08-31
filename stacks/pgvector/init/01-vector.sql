-- The image ships pgvector but does not install it: on a fresh database
-- `vector` is in pg_available_extensions with an empty installed_version.
-- This runs once, when the data volume is created.
CREATE EXTENSION IF NOT EXISTS vector;

-- Installing it into template1 as well means every database created later in
-- this container inherits it, so `CREATE DATABASE app` does not quietly hand
-- you a Postgres with no vector type.
\connect template1
CREATE EXTENSION IF NOT EXISTS vector;
