# @yesoreyeram/grafana-utils

A CLI tool for managing Grafana datasources and utilities.

## Installation

```bash
npm install -g @yesoreyeram/grafana-utils
# or use with npx
npx @yesoreyeram/grafana-utils --help
```

## Usage

### Global Commands

```bash
grafana-utils --help          # Show help
grafana-utils --version       # Show version
```

### Commands

#### setup-yarn

Setup and configure Yarn with a specific version using corepack.

```bash
# Use default version (4.16.0)
grafana-utils setup-yarn

# Specify a custom version
grafana-utils setup-yarn --yarn-version 4.20.0

# Short form
grafana-utils setup-yarn -v 4.20.0
```

##### What it does

1. Enables corepack
2. Prepares the specified Yarn version
3. Activates Yarn
4. Installs dependencies
5. Sets the Yarn version with yarn-path

##### Example with npx

```bash
npx @yesoreyeram/grafana-utils setup-yarn --yarn-version 4.16.0
# or with short flag
npx @yesoreyeram/grafana-utils setup-yarn -v 4.16.0
```

## Project

This project is part of [Grafana X](https://github.com/yesoreyeram/grafana-x) - Collection of Grafana plugins, datasources, panels, tools, skills, and experiments.
