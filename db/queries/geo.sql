-- name: ListCountries :many
SELECT id, name_ro, name_ru, name_en, iso3, iso2
FROM countries
ORDER BY id;

-- name: GetCountry :one
SELECT id, name_ro, name_ru, name_en, iso3, iso2
FROM countries
WHERE id = $1;

-- name: CreateCountry :one
INSERT INTO countries (name_ro, name_ru, name_en, iso3, iso2)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name_ro, name_ru, name_en, iso3, iso2;

-- name: UpdateCountry :one
UPDATE countries
SET name_ro = $2, name_ru = $3, name_en = $4, iso3 = $5, iso2 = $6
WHERE id = $1
RETURNING id, name_ro, name_ru, name_en, iso3, iso2;

-- name: DeleteCountry :execrows
DELETE FROM countries
WHERE id = $1;

-- name: ListCities :many
SELECT id, country_id, name_ro, name_ru, name_en, is_capital, population, timezone
FROM cities
ORDER BY id;

-- name: GetCity :one
SELECT id, country_id, name_ro, name_ru, name_en, is_capital, population, timezone
FROM cities
WHERE id = $1;

-- name: CreateCity :one
INSERT INTO cities (country_id, name_ro, name_ru, name_en, is_capital, population, timezone)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, country_id, name_ro, name_ru, name_en, is_capital, population, timezone;

-- name: UpdateCity :one
UPDATE cities
SET country_id = $2, name_ro = $3, name_ru = $4, name_en = $5, is_capital = $6, population = $7, timezone = $8
WHERE id = $1
RETURNING id, country_id, name_ro, name_ru, name_en, is_capital, population, timezone;

-- name: DeleteCity :execrows
DELETE FROM cities
WHERE id = $1;

-- name: ListAirports :many
SELECT id, city_id, iata_code, icao_code, lat, lon
FROM airports
ORDER BY id;

-- name: GetAirport :one
SELECT id, city_id, iata_code, icao_code, lat, lon
FROM airports
WHERE id = $1;

-- name: CreateAirport :one
INSERT INTO airports (city_id, iata_code, icao_code, lat, lon)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, city_id, iata_code, icao_code, lat, lon;

-- name: UpdateAirport :one
UPDATE airports
SET city_id = $2, iata_code = $3, icao_code = $4, lat = $5, lon = $6
WHERE id = $1
RETURNING id, city_id, iata_code, icao_code, lat, lon;

-- name: DeleteAirport :execrows
DELETE FROM airports
WHERE id = $1;

-- name: GetCitiesDropdown :many
SELECT
    c.id,
    (CASE WHEN @locale::text = 'ru' THEN c.name_ru WHEN @locale::text = 'ro' THEN c.name_ro ELSE c.name_en END)::text AS name,
    (CASE WHEN @locale::text = 'ru' THEN co.name_ru WHEN @locale::text = 'ro' THEN co.name_ro ELSE co.name_en END)::text AS country_name,
    a.iata_code AS airport_code
FROM cities c
JOIN countries co ON c.country_id = co.id
JOIN airports a ON a.city_id = c.id
WHERE
    (a.iata_code = UPPER(@search::text))
    OR ((@locale::text = 'ru' AND (c.name_ru % @search::text OR c.name_ru ILIKE CONCAT(@search::text, '%')))
        OR (@locale::text = 'ro' AND (c.name_ro % @search::text OR c.name_ro ILIKE CONCAT(@search::text, '%')))
        OR (@locale::text = 'en' AND (c.name_en % @search::text OR c.name_en ILIKE CONCAT(@search::text, '%'))))
    OR ((@locale::text = 'ru' AND (co.name_ru % @search::text OR co.name_ru ILIKE CONCAT(@search::text, '%')))
        OR (@locale::text = 'ro' AND (co.name_ro % @search::text OR co.name_ro ILIKE CONCAT(@search::text, '%')))
        OR (@locale::text = 'en' AND (co.name_en % @search::text OR co.name_en ILIKE CONCAT(@search::text, '%'))))
ORDER BY
    (a.iata_code = UPPER(@search::text)) DESC,
    GREATEST(
        similarity(CASE @locale::text WHEN 'ru' THEN c.name_ru WHEN 'ro' THEN c.name_ro ELSE c.name_en END, @search::text),
        similarity(CASE @locale::text WHEN 'ru' THEN co.name_ru WHEN 'ro' THEN co.name_ro ELSE co.name_en END, @search::text)
    ) DESC,
    c.population DESC NULLS LAST,
    c.is_capital DESC
LIMIT @limit_rows::bigint;
