DB_URL = postgresql://feedgrep:YyRzmj3dSdpnxK94@localhost:5432/feedgrep?sslmode=disable

migrate:
	goose -dir data/migrations postgres $(DB_URL) up 
	$(MAKE) schema
	
migrate-down:
	goose -dir data/migrations  postgres $(DB_URL) down 
	$(MAKE) schema
	
schema:
	mkdir -p ./data/schema && \
	pg_dump $(DB_URL) --schema-only | \
	grep -v -e '^--' -e '^COMMENT ON' -e '^REVOKE' -e '^GRANT' -e '^SET' -e 'ALTER DEFAULT PRIVILEGES' -e 'OWNER TO' | \
	cat -s > ./data/schema/schema.sql