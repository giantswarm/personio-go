# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]

### Fixed

- Fixed `GetEmployees()` only returning first page of data
- Fixed `GetTimeOffs()` date argument format (YYYY-MM-DD)

### Added

- Add `NewClientWithTimeout()` to allow specifying the client timeout
- Add `GetMapAttribute()` to simply get nested object's attributes as `map[string]interface{}`

## [0.0.1] - 2022-12-08

- Add `GetEmployees()` and `GetEmployee(id)` to handle `GET /company/employees`
- Add `GetTimeOffs()` to handle `GET /company/time-offs`
- Implement basic request handling and `accessToken` rotation
- Add `Authenticate()` to handle `POST /auth`
- Add Personio API v1 client

[Unreleased]: https://github.com/giantswarm/personio-go/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/giantswarm/personio-go/releases/tag/v0.0.1
