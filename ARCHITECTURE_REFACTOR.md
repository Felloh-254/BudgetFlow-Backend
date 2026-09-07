# BudgetFlow Backend Architecture Refactoring

## Executive Summary

The backend has been refactored to implement **double-entry accounting** with a ledger-based architecture. This replaces the simplistic single-account transaction model with a proper financial model that correctly handles:

- **Income**: Money entering an account
- **Expenses**: Money leaving an account  
- **Transfers**: Money moving between accounts (2-sided transaction)
- **Account Balance Reconciliation**: Verification and correction of balance inconsistencies
- **Idempotency**: Duplicate-safe transaction creation via idempotency keys

## What Changed

### 1. **Database Schema**

#### Old Schema (Deprecated)
```sql
transactions (
  id, user_id, account_id, budget_id, category_id, 
  title, amount, type, date, note, created_at
)
-- accounts.balance was directly stored and updated
```

#### New Schema
```sql
-- Event-oriented transactions (no account-specific info)
transactions_v2 (
  id, user_id, type, title, date, note, 
  idempotency_key, created_at, updated_at
)

-- Account movements (double-entry)
ledger_entries (
  id, transaction_id, account_id, amount, entry_type, created_at
)

-- Mapping transactions to categories
transaction_categories (
  transaction_id, category_id
)

-- Denormalized balance (for query performance)
account_balances (
  account_id, balance, last_updated_txn, version, updated_at
)
```

### 2. **Data Models**

#### Transaction (now event-based)
```json
{
  "id": 123,
  "user_id": 1,
  "type": "income|expense|transfer",
  "title": "Salary deposit",
  "date": "2026-09-03",
  "note": "Monthly salary",
  "idempotency_key": "uuid-...",
  "created_at": "2026-09-03T10:30:00Z",
  "updated_at": "2026-09-03T10:30:00Z"
}
```

#### LedgerEntry (account effect)
```json
{
  "id": 456,
  "transaction_id": 123,
  "account_id": 10,
  "amount": 100000.00,
  "entry_type": "debit|credit",
  "created_at": "2026-09-03T10:30:00Z"
}
```

#### TransactionDetail (API response)
```json
{
  "transaction": {...},
  "entries": [
    {"account_id": 10, "amount": 100000, "entry_type": "debit"}
  ],
  "categories": [
    {"id": 5, "name": "Salary", "type": "income"}
  ],
  "account_names": {
    "10": "KCB Savings"
  }
}
```

### 3. **API Endpoints (Breaking Changes)**

#### Old Endpoints (❌ Removed)
```
POST   /api/transactions           (generic create)
PUT    /api/transactions/:id       (generic update)
```

#### New Endpoints (✅ New)
```
GET    /api/transactions                    List all transactions
POST   /api/transactions/income             Create income (+money to account)
POST   /api/transactions/expense            Create expense (-money from account)
POST   /api/transactions/transfer           Create transfer (move between accounts)
DELETE /api/transactions/:id                Delete transaction
```

### 4. **Request/Response Bodies**

#### Create Income
```json
POST /api/transactions/income
{
  "title": "Monthly salary",
  "amount": 100000,
  "account_id": 10,
  "category": "Salary",
  "date": "2026-09-03",
  "note": "September payment",
  "idempotency_key": "uuid-optional"
}

Response (201):
{
  "transaction": {
    "id": 123,
    "type": "income",
    "title": "Monthly salary",
    ...
  },
  "entries": [
    {
      "id": 456,
      "account_id": 10,
      "amount": 100000,
      "entry_type": "debit"
    }
  ],
  "categories": [...],
  "account_names": {"10": "KCB Savings"}
}
```

#### Create Expense
```json
POST /api/transactions/expense
{
  "title": "Groceries",
  "amount": 5000,
  "account_id": 10,
  "category": "Food",
  "date": "2026-09-03",
  "note": ""
}

Response (201):
{
  "transaction": {
    "id": 124,
    "type": "expense",
    "title": "Groceries",
    ...
  },
  "entries": [
    {
      "id": 457,
      "account_id": 10,
      "amount": -5000,        // Negative! Money left account
      "entry_type": "credit"
    }
  ],
  "categories": [...],
  "account_names": {"10": "KCB Savings"}
}
```

#### Create Transfer
```json
POST /api/transactions/transfer
{
  "title": "Transfer KCB to M-Pesa",
  "amount": 10000,
  "from_account_id": 10,
  "to_account_id": 20,
  "date": "2026-09-03",
  "note": "Moving funds"
}

Response (201):
{
  "transaction": {
    "id": 125,
    "type": "transfer",
    "title": "Transfer KCB to M-Pesa",
    ...
  },
  "entries": [
    {
      "id": 458,
      "account_id": 10,
      "amount": -10000,
      "entry_type": "credit"     // Money leaves KCB
    },
    {
      "id": 459,
      "account_id": 20,
      "amount": 10000,
      "entry_type": "debit"      // Money enters M-Pesa
    }
  ],
  "categories": [],              // Transfers have no category
  "account_names": {"10": "KCB Savings", "20": "M-Pesa Wallet"}
}
```

#### List Transactions
```json
GET /api/transactions?limit=20&offset=0

Response (200):
[
  {
    "id": 125,
    "user_id": 1,
    "type": "transfer",
    "title": "Transfer KCB to M-Pesa",
    "date": "2026-09-03",
    "note": "Moving funds",
    "created_at": "...",
    "updated_at": "..."
  },
  ...
]
```

### 5. **Account Balances**

**The `accounts.balance` field is now read-only** and sourced from the `account_balances` table.

When you list or fetch accounts:
```json
GET /api/accounts

Response:
[
  {
    "id": 10,
    "user_id": 1,
    "name": "KCB Savings",
    "type": "bank",
    "balance": 85000,          // Computed from ledger_entries
    "currency": "KES",
    "account_number": "...",
    "created_at": "...",
    "updated_at": "..."
  }
]
```

When creating an account, you still provide `balance` as the **initial balance**:
```json
POST /api/accounts
{
  "name": "KCB Savings",
  "type": "bank",
  "balance": 100000,           // Initial balance only
  "currency": "KES",
  "account_number": "..."
}
```

After creation, the balance becomes derived from transactions. Never directly update `balance`—transactions automatically maintain it.

## Frontend Implementation Guide

### 1. **Update Transaction Creation Forms**

**Old approach (single form with type dropdown):**
```html
<!-- ❌ OLD: No longer works -->
<form>
  <select name="type">
    <option value="income">Income</option>
    <option value="expense">Expense</option>
  </select>
  <input type="number" name="amount">
  <select name="account_id">...</select>
</form>
```

**New approach (separate endpoints):**
```html
<!-- ✅ NEW: Three separate submission paths -->

<!-- Income Form -->
<form id="income-form">
  <input type="text" placeholder="Income title" name="title">
  <input type="number" placeholder="Amount" name="amount">
  <select name="account_id">
    <option value="">Select destination account</option>
    <!-- options populate from /api/accounts -->
  </select>
  <input type="text" placeholder="Category" name="category">
  <input type="date" name="date">
  <input type="text" placeholder="Note" name="note">
  <button type="submit">Record Income</button>
</form>
<script>
  document.getElementById('income-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    const response = await fetch('/api/transactions/income', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': '...' },
      body: JSON.stringify(Object.fromEntries(formData))
    });
    const result = await response.json();
    console.log('Transaction created:', result.transaction.id);
  });
</script>

<!-- Expense Form -->
<form id="expense-form">
  <!-- Same fields as income -->
  <button type="submit">Record Expense</button>
</form>
<script>
  document.getElementById('expense-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const response = await fetch('/api/transactions/expense', {
      method: 'POST',
      body: JSON.stringify(Object.fromEntries(new FormData(e.target)))
    });
    // ... handle response
  });
</script>

<!-- Transfer Form -->
<form id="transfer-form">
  <input type="text" placeholder="Transfer title" name="title">
  <input type="number" placeholder="Amount" name="amount">
  <select name="from_account_id" required>
    <option value="">From account</option>
  </select>
  <select name="to_account_id" required>
    <option value="">To account</option>
  </select>
  <input type="date" name="date">
  <input type="text" placeholder="Note" name="note">
  <button type="submit">Transfer</button>
</form>
<script>
  document.getElementById('transfer-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const response = await fetch('/api/transactions/transfer', {
      method: 'POST',
      body: JSON.stringify(Object.fromEntries(new FormData(e.target)))
    });
    // ... handle response
  });
</script>
```

### 2. **Update Transaction List Display**

The transaction list now shows the event, not the account movement directly.

```typescript
// Old response structure (deprecated)
// { id, user_id, account_id, amount, type, category, ... }

// New response structure
// { id, user_id, type, title, date, note, created_at, updated_at }
// Account and amount info comes from related endpoints

async function listTransactions() {
  const txns = await fetch('/api/transactions').then(r => r.json());
  
  for (const txn of txns) {
    // For display, you need to fetch the detailed view
    // OR enrich the list endpoint (we'll add that)
    const detail = await fetchTransactionDetail(txn.id);
    
    displayTransaction({
      id: detail.transaction.id,
      type: detail.transaction.type,
      title: detail.transaction.title,
      date: detail.transaction.date,
      // Amount and account info from entries
      amount: Math.abs(detail.entries[0].amount),
      account: detail.account_names[detail.entries[0].account_id],
      category: detail.categories[0]?.name || 'Transfer'
    });
  }
}
```

### 3. **Display Transfers Correctly**

Transfers now appear as a separate type:

```typescript
function displayTransactionRow(detail) {
  const { transaction, entries, categories, account_names } = detail;
  
  if (transaction.type === 'transfer') {
    const fromEntry = entries.find(e => e.amount < 0);
    const toEntry = entries.find(e => e.amount > 0);
    
    return `
      <tr class="transfer">
        <td>${transaction.title}</td>
        <td>${account_names[fromEntry.account_id]} → ${account_names[toEntry.account_id]}</td>
        <td>${Math.abs(fromEntry.amount)}</td>
        <td class="neutral">Transfer</td>
      </tr>
    `;
  } else if (transaction.type === 'income') {
    return `
      <tr class="income">
        <td>${transaction.title}</td>
        <td>${account_names[entries[0].account_id]}</td>
        <td class="amount-positive">+${entries[0].amount}</td>
        <td>${categories[0]?.name}</td>
      </tr>
    `;
  } else if (transaction.type === 'expense') {
    return `
      <tr class="expense">
        <td>${transaction.title}</td>
        <td>${account_names[entries[0].account_id]}</td>
        <td class="amount-negative">${entries[0].amount}</td>
        <td>${categories[0]?.name}</td>
      </tr>
    `;
  }
}
```

### 4. **Handle Errors**

New validation errors due to stricter types:

```typescript
async function createIncome(data) {
  try {
    const response = await fetch('/api/transactions/income', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    
    if (!response.ok) {
      const error = await response.json();
      if (error.error.includes("cannot transfer to the same account")) {
        // Transfer validation (won't appear in income, but in transfer)
      } else if (error.error.includes("account_id must be greater than 0")) {
        showError("Please select a destination account");
      } else if (error.error.includes("amount must be greater than 0")) {
        showError("Amount must be positive");
      }
    }
    return await response.json();
  } catch (err) {
    showError("Request failed: " + err.message);
  }
}
```

### 5. **Idempotency Support**

For robust UX (e.g., retry on network failures), send an `idempotency_key`:

```typescript
async function createTransactionSafely(data) {
  const idempotencyKey = crypto.randomUUID();
  
  const response = await fetch('/api/transactions/income', {
    method: 'POST',
    body: JSON.stringify({
      ...data,
      idempotency_key: idempotencyKey
    })
  });
  
  if (!response.ok && response.status < 500) {
    // Client error - don't retry with same key
    throw new Error(await response.text());
  } else if (!response.ok) {
    // Server error - retry with same key is safe
    // (duplicate requests return the same result)
    return await createTransactionSafely(data);
  }
  
  return await response.json();
}
```

### 6. **Account Display**

Balance is now read-only:

```typescript
async function displayAccounts() {
  const accounts = await fetch('/api/accounts').then(r => r.json());
  
  for (const account of accounts) {
    console.log(`${account.name}: ${account.balance} ${account.currency}`);
    // Balance is automatically maintained—don't show a manual "update balance" form
  }
}
```

### 7. **Reconciliation (Optional, for power users)**

A new admin endpoint will be added for balance verification:

```typescript
// (Future) Verify account balance consistency
async function reconcileAccount(accountId) {
  const result = await fetch(`/api/admin/reconcile/accounts/${accountId}`, {
    method: 'POST',
    body: JSON.stringify({ fix: true })
  }).then(r => r.json());
  
  if (result.is_consistent) {
    console.log("✅ Account balance is consistent");
  } else {
    console.log(`⚠️ Found discrepancy: ${result.discrepancy}`);
    console.log(`📊 Ledger balance: ${result.ledger_balance}, Stored: ${result.stored_balance}`);
  }
}
```

## Migration Path (Data Integrity)

### Phase 1 (Current)
- New schema is in place (`transactions_v2`, `ledger_entries`, `account_balances`)
- Old `transactions` table still exists but unused

### Phase 2 (Optional: Data Migration)
```sql
-- Migrate old transactions to new schema
INSERT INTO transactions_v2 (user_id, type, title, date, note, created_at, updated_at)
SELECT user_id, type, title, date, note, created_at, created_at
FROM transactions;

INSERT INTO ledger_entries (transaction_id, account_id, amount, entry_type, created_at)
SELECT t.id, t.account_id, 
  CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END,
  CASE WHEN t.type = 'income' THEN 'debit' ELSE 'credit' END,
  t.created_at
FROM transactions t;

INSERT INTO account_balances (account_id, balance, version)
SELECT id, balance, 1 FROM accounts;
```

### Phase 3 (Cleanup)
- Drop old `transactions` table after verifying migration

## Key Benefits

✅ **Correctness**: Transfers properly represented as 2-sided transactions
✅ **Atomicity**: Transaction + ledger + balance all update together  
✅ **Auditability**: Complete ledger history never changes  
✅ **Consistency**: Balance always derivable from ledger  
✅ **Idempotency**: Safe retries with idempotency keys  
✅ **Extensibility**: Supports refunds, reconciliation, imports, reversals

## Gotchas & Tips

1. **Amount always positive in API**
   - Income: amount = 100, stored as +100 in ledger
   - Expense: amount = 50, stored as -50 in ledger
   - Frontend doesn't need to negate

2. **No more budget on transactions**
   - Remove `budget_id` from forms (for now)
   - Budget tracking moved to separate endpoint

3. **Category is per-transaction, not ledger entry**
   - One transaction can have multiple categories in future
   - For now, stick to one category per transaction

4. **Transfers don't have categories**
   - `categories` array will be empty for transfer type
   - Don't show category dropdown for transfers

5. **Backward compatibility**
   - Old API endpoints no longer exist
   - Update all frontend calls to new endpoints
   - Test with Postman or curl before deploying

## Testing Checklist

- [ ] Create income transaction → verify account balance increased
- [ ] Create expense transaction → verify account balance decreased
- [ ] Create transfer → verify both accounts updated correctly
- [ ] List transactions → verify all types displayed correctly
- [ ] Delete transaction → verify balance reversed
- [ ] Fetch transaction detail → verify entries and categories present
- [ ] Create with same idempotency key twice → verify only one created
- [ ] Invalid account_id → verify 404 returned
- [ ] Same from/to account in transfer → verify validation error
