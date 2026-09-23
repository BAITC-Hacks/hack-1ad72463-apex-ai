CREATE TABLE IF NOT EXISTS catalog_state (
    id smallint PRIMARY KEY CHECK (id = 1),
    metadata jsonb NOT NULL,
    facts jsonb NOT NULL,
    vendor_count integer NOT NULL CHECK (vendor_count > 0),
    content_hash text NOT NULL,
    loaded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vendors (
    id text PRIMARY KEY,
    anon_name text NOT NULL CHECK (length(anon_name) > 0),
    city text NOT NULL,
    city_key text NOT NULL,
    categories text[] NOT NULL CHECK (cardinality(categories) > 0),
    category_keys text[] NOT NULL,
    city_imputed boolean NOT NULL,
    synthetic boolean NOT NULL,
    price_from_kzt bigint NOT NULL CHECK (price_from_kzt > 0),
    price_imputed boolean NOT NULL,
    event_formats text[] NOT NULL CHECK (cardinality(event_formats) > 0),
    languages text[] NOT NULL CHECK (cardinality(languages) > 0),
    max_hours double precision CHECK (max_hours > 0 AND max_hours < 'Infinity'::float8),
    busy_dates date[] NOT NULL,
    description text NOT NULL CHECK (length(description) > 0)
);
CREATE INDEX IF NOT EXISTS vendors_city_key_idx ON vendors (city_key);
CREATE INDEX IF NOT EXISTS vendors_categories_idx ON vendors USING gin (category_keys);
