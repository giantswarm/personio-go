# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]

## [0.5.0] - 2024-10-16

- Reduced pagingMaxLimit to 100 in order to comply with upcoming Personio API changes

## [0.4.0] - 2024-03-26

### Added

- Add `v1.GetTimeOffsMapped()` function

## [0.3.0] - 2023-07-05

### Added

- Add `TimeOff.Comment` field

## [0.2.0] - 2023-06-04

- Renamed `GetListValue()` to `GetTagValues()`
- Changed `GetStringValue()` to also read the value of `list` attributes

## [0.1.0] - 2023-02-22

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

[Unreleased]: https://github.com/giantswarm/personio-go/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/giantswarm/personio-go/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/giantswarm/personio-go/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/personio-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/giantswarm/personio-go/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/giantswarm/personio-go/releases/tag/v0.0.1
