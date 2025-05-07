-- Chart of Accounts
CREATE TABLE accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_code TEXT UNIQUE NOT NULL,      -- e.g., '1010', '4000', '5010'
    account_name TEXT NOT NULL,             -- e.g., 'Cash', 'Sales Revenue', 'Office Supplies Expense'
    account_type TEXT NOT NULL CHECK (account_type IN ('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE')), -- Basic account types
    normal_balance TEXT NOT NULL CHECK (normal_balance IN ('DEBIT', 'CREDIT')), -- Expected balance increase type
    is_active INTEGER NOT NULL DEFAULT 1,
    description TEXT
);

-- Journal Entries (Transactions)
CREATE TABLE journal_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_date DATE NOT NULL,
    description TEXT NOT NULL,              -- Description of the overall transaction
    reference TEXT,                         -- Optional reference (e.g., invoice number, check number)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Ledger Entries (Debits and Credits for each Journal Entry)
CREATE TABLE ledger_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    journal_entry_id INTEGER NOT NULL,
    account_id INTEGER NOT NULL,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('DEBIT', 'CREDIT')),
    amount REAL NOT NULL CHECK (amount > 0), -- Amount must be positive
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (journal_entry_id) REFERENCES journal_entries(id) ON DELETE CASCADE,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT -- Prevent deleting accounts with entries
);

-- Index for performance
CREATE INDEX idx_ledger_entries_journal ON ledger_entries(journal_entry_id);
CREATE INDEX idx_ledger_entries_account ON ledger_entries(account_id);

-- Constraint (requires triggers in SQLite) to ensure debits = credits per journal entry
-- Note: Enforcing this strictly in SQLite often requires application-level logic or complex triggers.
-- A simple check query:
-- SELECT journal_entry_id, SUM(CASE WHEN entry_type = 'DEBIT' THEN amount ELSE 0 END) as total_debits,
--        SUM(CASE WHEN entry_type = 'CREDIT' THEN amount ELSE 0 END) as total_credits
-- FROM ledger_entries
-- GROUP BY journal_entry_id
-- HAVING total_debits != total_credits;
