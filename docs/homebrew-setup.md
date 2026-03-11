# Homebrew Installation

## Quick Install

    brew tap saltrenis/tap https://github.com/Saltrenis/APIAudit
    brew install saltrenis/tap/apiaudit

## Upgrading

    brew upgrade saltrenis/tap/apiaudit

## Migration Note

For a proper Homebrew tap, the formula should live in a separate repository
named `Saltrenis/homebrew-tap`. To migrate:

1. Create the repo `Saltrenis/homebrew-tap`
2. Move `Formula/apiaudit.rb` there
3. Users install with: `brew tap saltrenis/tap && brew install apiaudit`
