ALTER TABLE urls
    ADD CONSTRAINT urls_original_url_key UNIQUE (original_url);