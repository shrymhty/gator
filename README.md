# Gator: CLI RSS Aggregator

Gator is a command-line interface tool (CLI) written in Go that allows users to login, register, follow RSS feeds, and aggregate posts from those feeds into PostgreSQL database.

## Features
- **User Management**: Create accounts and log in to manage your feed subscriptions.
- **RSS Aggregation**: Automatically scrape and store posts from your favorite RSS feeds.
- **Feed Control**: Follow and unfollow feeds with simple CLI commands.
- **Browsing**: View the latest aggregated posts for your account.

## Prerequisites
- Go 1.20+
- PostgreSQL
- `sqlc` (for database code generation)

## Installation

### Using go install (Recommended)
You can install the latest version directly from GitHub:

```bash
go install [github.com/shrymhty/gator@latest](https://github.com/shrymhty/gator@latest)
```

### Building from source
#### If you are developing locally:
1. Clone the repository:

```bash
git clone [https://github.com/shrymhty/gator](https://github.com/shrymhty/gator)
cd gator
```

2. Build the binary

```bash
go build -o gator .
```

## Usage

```bash
./gator help
```

### Common Commands

- Register a new user
```bash
./gator register <username>
```

- Add and follow a feed:
```bash
./gator addfeed <name> <url>
```

- Start the aggregator:
```bash
./gator agg 1m  # Fetches every 1 minute
```

- Browse your aggregated posts:
```bash
./gator browse <limit>
```