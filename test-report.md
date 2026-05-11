# Test Report — IEC 60870-5-104 Simulator

**Date:** 2026-05-10  
**Go:** go1.21.13 linux/amd64  
**Binary:** 7,758,224 bytes (stripped, dynamically linked)

---

## Results

| Phase | Status |
|-------|--------|
| `go vet ./...` | **PASS** |
| Unit Tests (4 packages) | **PASS** — 27/27 passed |
| Race Detection | **PASS** — 4/4 packages clean |
| Build Check | **PASS** |
| Integration Test (HTTP API) | **PASS** — 19/19 passed |

---

## Unit Tests — Package Breakdown

### `iec104-sim/api` — 9 tests (0.012s)
| Test | Result |
|------|--------|
| `TestListPoints` | PASS |
| `TestGetPoint` | PASS |
| `TestGetPoint_NotFound` | PASS |
| `TestUpdatePoint_AI` | PASS |
| `TestUpdatePoint_DI` | PASS |
| `TestUpdatePoint_PI` | PASS |
| `TestBatchUpdate` | PASS |
| `TestUpdateQDS` | PASS |
| `TestStatus` | PASS |

### `iec104-sim/config` — 3 tests (0.035s)
| Test | Result |
|------|--------|
| `TestLoadFromXLSX_ValidFile` | PASS |
| `TestLoadFromXLSX_DuplicateIOA` | PASS |
| `TestLoadFromXLSX_InvalidPointType` | PASS |

### `iec104-sim/iec104` — 7 tests (2.219s)
| Test | Result |
|------|--------|
| `TestServer_StartStop` | PASS |
| `TestServer_SingleClient` | PASS |
| `TestServer_PublishIntegration` | PASS |
| `TestServer_Stats` | PASS |
| `TestStorePublishFlow` | PASS |
| `TestServer_DataRace` | PASS |
| `TestServer_APCIFrame` | PASS |

### `iec104-sim/library` — 9 tests (0.009s)
| Test | Result |
|------|--------|
| `TestNewStore` | PASS |
| `TestStore_Get` | PASS |
| `TestStore_SetValue` | PASS |
| `TestStore_SetBoolValue` | PASS |
| `TestStore_SetIntValue` | PASS |
| `TestStore_CollectChanged` | PASS |
| `TestStore_GetByType` | PASS |
| `TestStore_CountByType` | PASS |
| `TestStore_SetQDS` | PASS |

---

## Integration Tests — 19 HTTP End-to-End

| # | Test | Result |
|---|------|--------|
| 1 | `GET /api/status` — total_points | PASS |
| 2 | `GET /api/status` — version | PASS |
| 3 | `GET /api/points` — returns points | PASS |
| 4 | `GET /api/points` — count = 7 | PASS |
| 5 | `GET /api/points/5` — DI_01 exists | PASS |
| 6 | `GET /api/points/5` — type=DI | PASS |
| 7 | `GET /api/points/9999` — 404 | PASS |
| 8 | `PUT /api/points/16385` — AI update | PASS |
| 9 | `PUT /api/points/16385` — value updated | PASS |
| 10 | `PUT /api/points/5` — DI update | PASS |
| 11 | `PUT /api/points/5` — bool_value=true | PASS |
| 12 | `POST /api/points` — batch updated=2 | PASS |
| 13 | `POST /api/points` — batch AI=999.99 | PASS |
| 14 | `PUT /api/points/16385/qds` — QDS update | PASS |
| 15 | `PUT /api/points/16385/qds` — invalid=true | PASS |
| 16 | `PUT /api/points/16385/qds` — blocked=true | PASS |
| 17 | `GET /api/status` — client_connected | PASS |
| 18 | `GET /api/status` — point_counts | PASS |
| 19 | Type safety: PUT bool on AI point | PASS |

---

## Changes Made in This Session

1. **`iec104/server_test.go`** — Fixed `findFreePort` to accept `testing.TB` (supports both `*testing.T` and `*testing.B`); removed unused `srv` variable in `TestServer_APCIFrame`; added `time.Sleep(100ms)` after `Start()` to avoid "connection refused" from async server startup.

2. **`iec104/server.go`** — Replaced `atomic.Value` + `atomic.Bool` with `sync.RWMutex`-protected plain `asdu.Connect` + `bool` fields. The prior approach used `atomic.Value` which panicked on type inconsistency when `onConnect` stored a concrete `asdu.Connect` type (e.g., `*cs104.SrvSession`) and `onDisconnect` stored a different concrete type.

3. **`full_test.sh`** — Changed working directory from `~/iec104-sim` (WSL home copy) to `/mnt/d/AI/Claw/iec104-sim` (Windows mount) to eliminate stale-file sync issues.
