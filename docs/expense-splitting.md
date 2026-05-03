# Expense Splitting

All monetary values in the service layer are `int64` **minor units** (cents for most currencies, whole units for JPY). The handler converts from human-readable `float64` on input and back on output using `domain.MinorUnitFactor(groupCurrency)`.

---

## Split Methods

### Equal

All group members share the expense equally. Integer division is used throughout — the last member absorbs any indivisible remainder (at most `n-1` minor units for a group of `n` members).

```
Amount: ¥10000 (JPY, factor=1), Members: 3
→ share = 10000 / 3 = 3333
→ Member 1: 3333
→ Member 2: 3333
→ Member 3: 3334  ← absorbs remainder (10000 - 3333×2)
Sum: 10000 ✓

Amount: $100.00 (USD, stored as 10000 cents), Members: 3
→ share = 10000 / 3 = 3333 cents ($33.33)
→ Member 1: 3333 cents
→ Member 2: 3333 cents
→ Member 3: 3334 cents  ← absorbs 1 cent remainder
Sum: 10000 cents = $100.00 ✓
```

**Request:** No `splits` map needed.
```json
{ "amount": 100.00, "split_method": "equal" }
```

---

### Exact

Each member's share is specified explicitly in the `splits` map as `{ "user_id": amount }`. The sum of all amounts must equal the expense amount **exactly** (integer comparison after converting to minor units).

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
  "splits": {
    "uuid-jane": 50.00,
    "uuid-bob":  40.00
  }
}
```

**Validation error** if `sum(splits) ≠ amount` after conversion to minor units.

---

### Percentage

Each member's share is specified as a percentage in the `splits` map as `{ "user_id": percentage }`. Percentages must sum to exactly **100**.

Each share is calculated as `int64(math.Round(amount_minor × pct / 100.0))`. Any rounding remainder is absorbed by the member with the largest percentage.

```
Amount: $200.00 (20000 cents), JPY ¥20000
→ Jane: 60% → int64(Round(20000 × 60/100)) = 12000
→ Bob:  40% → int64(Round(20000 × 40/100)) = 8000
Sum: 20000 ✓
```

**Request:**
```json
{
  "amount": 200.00,
  "split_method": "percentage",
  "splits": {
    "uuid-jane": 60,
    "uuid-bob":  40
  }
}
```

**Validation error** if `sum(percentages) ≠ 100`.

---

## Implementation

**File:** `internal/expense/service.go::computeSplits()`

The handler converts the client's `float64` amount to `int64` minor units before calling the service. The service works exclusively in `int64`:

- **Equal:** `share = amount / n`, `remainder = amount - share*n` → added to last member
- **Exact:** sum check is `total == amount` (exact integer equality)
- **Percentage:** `math.Round` used only for the floating-point percentage multiplication; result cast to `int64`; remainder (if any) absorbed by the highest-percentage member

---

## Balance Calculation

**File:** `internal/expense/balance.go`

After expenses are created, the net balance for each member is computed with a single aggregating SQL query using a CTE:

```
NetBalance(user) = SUM(splits in expenses paid by user, for others)
                - SUM(splits owed by user, in expenses paid by others)
```

Result is `int64` minor units.

- `NetBalance > 0`: others owe this person
- `NetBalance < 0`: this person owes others
- `NetBalance = 0`: settled up

---

## Debt Simplification

**File:** `internal/expense/balance.go::SimplifyDebts()`

Raw per-member balances may require O(n) transactions (e.g., A→B, B→C, C→A). The greedy min-cash-flow algorithm reduces this to the minimum number of settlement transactions.

### Algorithm

```
Input: [Jane: +4500, Bob: -3000, Carol: -1500]  (minor units)

Step 1: Max creditor = Jane (+4500), max debtor = Bob (-3000)
Step 2: Settle min(4500, 3000) = 3000
        → Bob pays Jane 3000
        → Jane: +1500, Bob: 0, Carol: -1500

Step 3: Max creditor = Jane (+1500), max debtor = Carol (-1500)
Step 4: Settle min(1500, 1500) = 1500
        → Carol pays Jane 1500
        → All zeroed out

Result: 2 transactions (vs up to 3 without simplification)
```

### Termination condition

`debtor.bal == 0 && creditor.bal == 0` (exact integer equality, no epsilon).

### Complexity

- Time: O(n²) worst case, where n = number of members
- Acceptable in practice (group sizes typically < 20 members)

### Result

```go
[]DebtSummary{
    { FromUserName: "Bob",   ToUserName: "Jane",  Amount: 3000 },  // int64 minor units
    { FromUserName: "Carol", ToUserName: "Jane",  Amount: 1500 },
}
```

The `simplified_debts` array in the balance API response converts these values to human-readable amounts using the group's currency factor.

---

## Transaction Safety

Expense creation uses a database transaction:

1. `INSERT INTO expenses` — creates the expense record
2. Batch `INSERT INTO expense_splits` — creates all split records atomically via pgx `SendBatch`

If any insert fails, the entire transaction rolls back. There is no state where an expense exists without its splits, or vice versa.
