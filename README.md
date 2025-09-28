# Tender Scraper

A Go-based automated tender scraper designed to extract, process, and store tender information from multiple e-procurement platforms. Includes features for handling document downloads, captcha solving, retry mechanisms, and logging failed extractions for reliability.

---

## Features

* **Concurrent tender scraping** across multiple domains and states.
* **Detailed data extraction** including:

  * Basic details
  * Critical dates
  * Work/item information
  * Tender documents
  * Payment and EMD fee details
  * Corrigendum and authority information
* **Retry and failure handling** with logging for failed tenders.
* **Document downloads** with offline and online instrument support.
* **CSV and JSONL exports** for structured data persistence.
* **Session management** for stateful scraping.
* **Captcha solving support** for protected portals.

---

## Repo Structure

```
.
├── cli                    # Command-line entry points
├── docDownloads           # Handles tender document downloads and AWS integration
├── http                   # HTTP server utilities (optional API or monitoring)
├── scraper                # Core scraping logic
│   ├── captcha            # Captcha solver
│   ├── extract            # Tender data extraction and parsing
│   ├── nav                # Navigation and link extraction
│   └── pastTenders        # Historical tender processing
├── session                # Session management and state persistence
├── TenderData             # Output directories for logs, links, and tenders
└── utils                  # Shared utilities, types, and helpers
```

---

## Getting Started

### Prerequisites

* Go 1.21+
* Docker (optional, for containerized execution)
* Internet access to target tender portals

### Installation

Clone the repository:

```bash
git clone https://github.com/yourusername/tender-scraper.git
cd tender-scraper
go mod tidy
```

### Running

**CLI mode:**

```bash
go run cli/main.go
```

**Docker mode:**

```bash
docker build -t tender-scraper .
docker-compose up
```

### Configuration

* **Domains & States:** Specify in your CLI input or configuration files.
* **Output Path:** TendertData/ directory contains:

  * `Tenders/` → extracted tender data (JSONL)
  * `Links/` → scraped tender links (CSV)
  * `Failed/` → failed extractions
  * `Logs/` → scraping and session logs

---

## Core Components

### Scraper

* **DataScraper:** Manages a session and extracts single tender data.
* **TenderParser:** Handles parsing of tender pages using Colly.
* **Navigation:** Crawls links to tender pages and corrigenda.

### Failed Tender Handling

* **Failed Logs:** Automatically writes tenders or search links that fail extraction.
* **Retry Mechanism:** Up to 3 attempts with exponential backoff per tender.

### Document Downloads

* Supports downloading tender-related PDFs and files.
* Handles offline and online payment instrument details for document access.

### Utilities

* Conversion of extracted data to a consistent `utils.Tender` format.
* Date parsing, string cleanup, and session helpers.

---

## Logging

* Collector logs: `TenderData/Logs/collectors`
* Session logs: `TenderData/Logs/sessions`
* Failed tenders: `TenderData/Failed/`

All logs include timestamps, domain/state info, and serial number for traceability.
