# Changelog

## 0.1.6

Initial public release. Includes native Mediux metadata enrichment and image resolution, ordered creator search/preferences, independent season and title-card consistency policies, configurable gap handling, and Silo-themed administrator settings.

- Fix title-card language selection within the selected set. Creator priority remains ahead of language during fallback and independent selection.
- Preserve supported regional language codes such as `pt-BR` and `zh-CN` during language matching.
- Add portable browser checks, reproducible release packaging and GitHub CI.
- Document manual-gallery compatibility, Mediux-based coverage and remaining platform/cache validation limits.

## Earlier local versions

0.1.5 introduced Silo-themed settings. 0.1.4 introduced explicit creator-first consistency. 0.1.3 moved creator and artwork preferences to a plugin-owned page with token-preserving saves. These versions were local deployments, not public GitHub releases.
