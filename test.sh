#!/bin/bash

golangci-lint run -E goimports -E maligned -E unconvert -E interfacer ./...