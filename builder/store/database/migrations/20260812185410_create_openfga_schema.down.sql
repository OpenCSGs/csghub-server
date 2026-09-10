-- Remove the OpenFGA storage schema in dependency order.

DROP TABLE IF EXISTS changelog;

--bun:split

DROP TABLE IF EXISTS assertion;

--bun:split

DROP TABLE IF EXISTS authorization_model;

--bun:split

DROP TABLE IF EXISTS tuple;

--bun:split

DROP TABLE IF EXISTS store;
