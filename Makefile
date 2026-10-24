# Корневой Makefile: сценарии для показа. Сборка и проверки сервера — в backend/Makefile.

.PHONY: burst

# burst — залп событий по стенду (docs/load/burst.sh): make burst N="3000 10000" ADMIN_PASSWORD=… METRICS_TOKEN=…
# Локально: docker compose up -d --wait, затем загрузить снимок (SOURCE_TOKEN, SNAPSHOT) или взять встроенный.
burst:
	docs/load/burst.sh $(or $(N),3000 10000 50000)
