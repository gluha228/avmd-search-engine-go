CREATE TABLE IF NOT EXISTS countries (
    id BIGSERIAL PRIMARY KEY,
    name_ro VARCHAR(255) NOT NULL,
    name_ru VARCHAR(255) NOT NULL,
    name_en VARCHAR(255) NOT NULL,
    iso3 VARCHAR(3) NOT NULL,
    iso2 VARCHAR(2) NOT NULL
);

CREATE TABLE IF NOT EXISTS cities (
    id BIGSERIAL PRIMARY KEY,
    country_id BIGINT NOT NULL REFERENCES countries(id),
    name_ro VARCHAR(255) NOT NULL,
    name_ru VARCHAR(255) NOT NULL,
    name_en VARCHAR(255) NOT NULL,
    is_capital BOOLEAN NOT NULL DEFAULT FALSE,
    population BIGINT,
    timezone VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS airports (
    id BIGSERIAL PRIMARY KEY,
    city_id BIGINT NOT NULL REFERENCES cities(id),
    iata_code VARCHAR(10),
    icao_code VARCHAR(10),
    lat DOUBLE PRECISION,
    lon DOUBLE PRECISION
);

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_cities_name_ru_trgm ON cities USING gin (name_ru gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cities_name_ro_trgm ON cities USING gin (name_ro gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cities_name_en_trgm ON cities USING gin (name_en gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_countries_name_ru_trgm ON countries USING gin (name_ru gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_countries_name_ro_trgm ON countries USING gin (name_ro gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_countries_name_en_trgm ON countries USING gin (name_en gin_trgm_ops);
