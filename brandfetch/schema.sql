-- Base table: raw incoming transaction strings
create table if not exists raw_transactions (
  id uuid primary key default gen_random_uuid(),
  description text not null unique,
  processed boolean default false,
  created_at timestamp with time zone default now()
);

-- Enriched “sheet” table (single source of truth)
create table if not exists enriched_merchants (
  id uuid primary key default gen_random_uuid(),
  transaction_cache text not null unique, -- copy of raw description, used as unique key
  brand_name text,
  legal_name text,
  logo text,
  anzsic_class_code text,
  abn_head_office text,
  acn_head_office text,
  head_office_address text,
  website_url text,
  bpay_biller_code text,
  mcc_code_test text,
  wemoney_category text,
  confidence_score float,
  brandfetch_id text,
  full_response jsonb,
  created_at timestamp with time zone default now()
);

-- -------------------------------------------------------------------
-- Migration helper (run in Supabase SQL editor if the table already exists)
-- -------------------------------------------------------------------
-- 1) Ensure transaction_cache exists and is populated from old column
--    alter table enriched_merchants add column if not exists transaction_cache text;
--    update enriched_merchants set transaction_cache = coalesce(transaction_cache, raw_transaction_label);
-- 2) Drop the old duplicate column
--    alter table enriched_merchants drop column if exists raw_transaction_label;
-- 3) Enforce uniqueness on transaction_cache
--    alter table enriched_merchants drop constraint if exists enriched_merchants_transaction_cache_key;
--    alter table enriched_merchants add constraint enriched_merchants_transaction_cache_key unique (transaction_cache);
-- 4) Optional cleanup of nulls
--    delete from enriched_merchants where brand_name is null or website_url is null;

-- Optional add/remove columns later:
--    alter table enriched_merchants add column if not exists new_col text;
--    alter table enriched_merchants drop column if exists old_col;
