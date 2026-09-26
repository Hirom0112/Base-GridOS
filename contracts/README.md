# Contracts

Lint contracts with `buf lint contracts` and generate uncommitted language
bindings with `buf generate contracts`.

Before merging a contract change, check file-level compatibility against main:

```sh
buf breaking contracts --against '.git#branch=main'
```
