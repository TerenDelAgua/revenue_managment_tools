# TEREN HOTELS · TEST STRATEGY & CHECKLIST
## Feature: Floor Map Builder (FMB-001)
**Version:** 1.0.0  
**Date:** May 27, 2026  
**Status:** Pending Execution  
**Focus:** Backend Logic, Frontend UI/UX, System Integrity

---

## 1. Backend Tests (Go)
**Objective:** Verify business rules, database constraints, and API contracts.
**Tooling:** `go test`, `testcontainers` (Postgres), `httptest`.

### 1.1 Inventory Service (`GetMapWithAvailability`)
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **BT-01** | `GetMap_AvailableRoom` | Query map for date where room has NO bookings/blocks. | `availability` = "available", `active_booking` = null. | P0 |
| **BT-02** | `GetMap_OccupiedRoom` | Query map where room has `status='checked_in'` booking. | `availability` = "occupied", `active_booking` = UUID. | P0 |
| **BT-03** | `GetMap_PendingRoom` | Query map where room has `status='confirmed'` booking. | `availability` = "pending", `pending_booking` = UUID. | P0 |
| **BT-04** | `GetMap_BlockedRoom` | Query map where room has `room_blocks` overlapping dates. | `availability` = "blocked", `block` = UUID. | P0 |
| **BT-05** | `GetMap_PriorityLogic` | Room has BOTH active booking AND block on same date. | `availability` = "occupied" (Booking > Block). | P1 |
| **BT-06** | `GetMap_InactiveRoom` | Room `status='inactive'`. | `availability` = "inactive" regardless of bookings. | P1 |

### 1.2 Booking Service (Assignment & Status)
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **BT-07** | `Assign_RoomAvailable` | Assign booking to "available" room. | Success (201). Booking created/updated. | P0 |
| **BT-08** | `Assign_RoomOccupied` | Assign booking to "occupied" room. | Error (409 Conflict). "Room unavailable". | P0 |
| **BT-09** | `Assign_RoomBlocked` | Assign booking to "blocked" room. | Error (409 Conflict). "Room blocked". | P0 |
| **BT-10** | `CheckIn_Flow` | Call CheckIn on confirmed booking. | Status changes to `checked_in`. | P0 |
| **BT-11** | `CheckOut_Flow` | Call CheckOut on checked_in booking. | Status changes to `checked_out`. | P0 |

### 1.3 Database Constraints (Integrity)
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **BT-12** | `UniquePosition` | Create Room A at (0,0). Create Room B at (0,0) same floor. | DB Error (Unique Violation). API returns 409. | P0 |
| **BT-13** | `UniqueRoomNumber` | Create Room "101" on Floor 1. Create "101" on Floor 2. | DB Error (Unique Violation). | P0 |
| **BT-14** | `ValidCoordinates` | Create Room with `pos_x: 15` (Out of bounds 0-11). | API returns 422 Unprocessable Entity. | P1 |
| **BT-15** | `BlockOverlap` | Block Room for May 27-28. Try Block May 27-29. | Error (409). | P1 |

---

## 2. Frontend Tests (Svelte 5 + Playwright/Vitest)
**Objective:** Verify UI rendering, component interaction, and Design System compliance.
**Tooling:** `Vitest` (Unit), `Playwright` (E2E/Integration).

### 2.1 Component Visuals & States
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **FT-01** | `Token_Available` | Render token with `availability="available"`. | Green bg (`#16A34A`), clean, no pattern. | P0 |
| **FT-02** | `Token_Occupied` | Render token with `availability="occupied"`. | Red bg (`#DC2626`), Bed icon. | P0 |
| **FT-03** | `Token_Blocked` | Render token with `availability="blocked"`. | Dark bg (`#44403C`), Striped pattern, Wrench icon. | P0 |
| **FT-04** | `DateInput_Clean` | Render `DateInput` component. | Native browser calendar icon hidden, TEREN icon visible. | P1 |
| **FT-05** | `Drawer_Context` | Open Drawer for Occupied Room. | Primary button shows "Check Out" (Black). | P0 |
| **FT-06** | `Drawer_Context` | Open Drawer for Available Room. | Primary button shows "Assign Booking" (Orange). | P0 |

### 2.2 User Interaction
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **FT-07** | `Drawer_SlideIn` | Click on Room Token. | Drawer slides in from right. Backdrop blur appears. | P0 |
| **FT-08** | `Drawer_SlideOut` | Click Backdrop or X button. | Drawer slides out. Map visible. | P0 |
| **FT-09** | `Block_Form_Inline` | Click "Block Room" inside Drawer. | Form expands inline. No modal popup. | P0 |
| **FT-10** | `Drag_Setup` | Drag token in Setup Mode. | Ghost image follows cursor. Drop zone highlights. | P1 |
| **FT-11** | `Inline_Edit` | Click Room Name in Setup Mode. | Text becomes input field. | P1 |

---

## 3. Integrity & Flow Tests (E2E)
**Objective:** Verify the complete loop from UI Action -> API -> DB -> UI Update.
**Tooling:** `Playwright` (Browser Automation).

### 3.1 Setup Flow (Owner)
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **IT-01** | `SaveLayout_Persist` | 1. Move Room 101 to (5,5).<br>2. Click Save.<br>3. Reload Page. | Room 101 remains at (5,5). | P0 |
| **IT-02** | `CreateRoom_Grid` | 1. Drag new room type to Grid.<br>2. Name it "New".<br>3. Save. | New room appears in DB and Map. | P1 |

### 3.2 Operations Flow (Receptionist)
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **IT-03** | `Assign_Update` | 1. Open Green Room.<br>2. Click Assign.<br>3. Select Booking. | Room turns Amber (Pending). API updated. | P0 |
| **IT-04** | `CheckIn_Update` | 1. Open Amber Room.<br>2. Click Check In. | Room turns Red (Occupied). Booking status=checked_in. | P0 |
| **IT-05** | `Block_Update` | 1. Open Green Room.<br>2. Fill Block Form.<br>3. Confirm. | Room turns Grey (Blocked). Token striped. | P0 |
| **IT-06** | `Optimistic_UI` | 1. Click Check In (Simulate slow network). | UI turns Red immediately. Reverts if API fails. | P1 |

### 3.3 Design System & Philosophy
| ID | Test Name | Scenario / Input | Expected Result | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **IT-07** | `No_Modals` | Perform all actions (Assign, Block, Edit). | Zero browser native alerts or modals used. | P0 |
| **IT-08** | `Contrast_Check` | View app on high brightness monitor. | Text readable on `#F5F4F1` background. | P1 |
| **IT-09** | `Mobile_Viewport` | Resize browser to 768px width. | Grid scrollable or responsive. No horizontal overflow. | P2 |

---

## 4. Performance Tests
| ID | Test Name | Metric | Threshold | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **PF-01** | `MapLoad_50Rooms` | Time to first paint of Grid. | < 1.5s (AC-09). | P0 |
| **PF-02** | `API_Latency_Map` | GET /map response time. | < 200ms (Server side). | P0 |
| **PF-03** | `Batch_Update` | PATCH /positions (50 items). | < 500ms. | P1 |

---

## ✅ Execution Log
*Date | Tester | Result | Notes*
*---|---|---|---*
*| | | |*
*| | | |*
