# Expense Splitting

## Split Methods

### Equal

All group members share the expense equally. The last member absorbs any rounding remainder to ensure the total always equals the original amount.

```
Amount: $100.00, Members: 3
→ Member 1: $33.33
→ Member 2: $33.33
→ Member 3: $33.34  ← absorbs rounding
Sum: $100.00 ✓
```

**Request:** No `splits` array needed.
```json
{ "amount": 100.00, "split_method": "equal" }
```

---

### Exact

Each member's share is specified explicitly. The sum of all amounts must equal the expense amount (tolerance: ±$0.01).

```
Amount: $90.00
→ Jane:  $50.00
→ Bob:   $40.00
Sum: $90.00 ✓
```

**Request:**
```json
{
  "amount": 90.00,
  "split_method": "exact",
  "splits": [
    { "user_id": "uuid-jane", "amount": 50.00 },
    { "user_id": "uuid-bob",  "amount": 40.00 }
  ]
}
```

**Validation error** if `sum(amounts) ≠ expense_amount ± 0.01`.

---

### Percentage

Each member's share is specified as a percentage. Percentages must sum to 100 (tolerance: ±0.01). Each share is calculated as `amount × percentage / 100`, rounded to cents.

```
Amount: $200.00
→ Jane:  60% → $120.00
→ Bob:   40% → $80.00
Sum: $200.00 ✓
```

**Request:**
```json
{
  "amount": 200.00,
  "split_method": "percentage",
  "splits": [
    { "user_id": "uuid-jane", "percentage": 60 },
    { "user_id": "uuid-bob",  "percentage": 40 }
  ]
}
```

**Validation error** if `sum(percentages) ≠ 100 ± 0.01`.

---

## Implementation

**File:** `internal/expense/service.go::computeSplits()`

All amounts are converted to integer cents before any arithmetic to avoid floating-point errors. The service layer receives `float64` from the API but immediately converts to `int64` cents for computation.

---

## Balance Calculation

**File:** `internal/expense/balance.go`

After expenses are created, the net balance for each member is computed using a single SQL query with a CTE:

```
NetBalance(user) = SUM(amount paid by user) - SUM(amount owed by user)
```

- `NetBalance > 0`: others owe this person
- `NetBalance < 0`: this person owes others
- `NetBalance = 0`: settled up

---

## Debt Simplification

**File:** `internal/expense/balance.go::SimplifyDebts()`

Raw balances may require O(n) transactions (e.g., A→B, B→C, C→A). The greedy min-cash-flow algorithm reduces this to the minimum number of transactions.

### Algorithm

```
Input: [Jane: +$45, Bob: -$30, Carol: -$15]

Step 1: Find max creditor (Jane: +$45) and max debtor (Bob: -$30)
Step 2: Settle min(45, 30) = $30
        → Bob pays Jane $30
        → Jane: +$15, Bob: $0, Carol: -$15

Step 3: Find max creditor (Jane: +$15) and max debtor (Carol: -$15)
Step 4: Settle min(15, 15) = $15
        → Carol pays Jane $15
        → All zeroed out

Result: 2 transactions instead of potentially 3
```

### Complexity

- Time: O(n²) in the worst case, where n = number of members
- This is acceptable because group sizes are small in practice (typically < 20 members)

### Result Shape

```go
[]DebtSummary{
    { From: "Bob",   To: "Jane",  Amount: 30.00 },
    { From: "Carol", To: "Jane",  Amount: 15.00 },
}
```

The `settlements` array in the balance API response is this simplified list.

---

## Transaction Safety

Expense creation uses a database transaction:

1. `INSERT INTO expenses` — creates the expense record
2. Batch `INSERT INTO expense_splits` — creates all split records atomically

If any insert fails, the entire transaction rolls back. There is no state where an expense exists without its splits, or vice versa.
