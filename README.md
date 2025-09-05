## File Structure
```text
main.go
scraper/
 ├── scraper.go        # Core: newCollector(), common helpers, shared types
 ├── portal_scraper.go # Logic for NICGeP-like portals (select & scrape choice)
 └── extract.go        # Data extraction (parse table rows into structs)
```
