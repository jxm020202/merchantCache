import pathlib
from typing import Optional, List, Dict, Any

import requests
from supabase import create_client, Client

# --- CONFIGURATION ---
BRANDFETCH_API_KEY = "DMQ1-j64ynSXKu2DeAWTuDtOYD8cWcPnCaXEkx3arsCUkIUoEoAt-_D-lhehcvnkBQ_Tm4yQ8bUmmbfHeNp5tQ"
BRANDFETCH_CLIENT_ID = "1idQrH5qMseJHQWSprB"
COUNTRY_TLD_PREFERENCE = ".au"  # prefer Australian domains when available
COUNTRY_CODE = "AU"             # target country for company/location filtering hints
SUPABASE_URL = "https://ecvosvvyqeqxigqykehd.supabase.co"
SUPABASE_SERVICE_ROLE_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImVjdm9zdnZ5cWVxeGlncXlrZWhkIiwicm9sZSI6InNlcnZpY2Vfcm9sZSIsImlhdCI6MTc2ODc5MTM1MiwiZXhwIjoyMDg0MzY3MzUyfQ.K0LchbdzwHncvz6SIH8TrmBu-w47tnVZMGK3JIGBVH0"

TRANSACTIONS_FILE = pathlib.Path(__file__).parent / "transactions.txt"

# Initialize Supabase
supabase: Client = create_client(SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY)


def load_transactions(file_path: pathlib.Path = TRANSACTIONS_FILE) -> List[str]:
    """Read transactions from a newline-delimited file."""
    if not file_path.exists():
        print(f"Transaction file not found at {file_path}. Add lines (one per transaction).")
        return []
    with file_path.open("r", encoding="utf-8") as f:
        lines = [line.strip() for line in f.readlines()]
    # Drop empty lines and de-duplicate while preserving order.
    seen = set()
    result = []
    for line in lines:
        if line and line not in seen:
            seen.add(line)
            result.append(line)
    return result


def get_logo_url(domain: Optional[str]) -> Optional[str]:
    if not domain:
        return None
    return f"https://cdn.brandfetch.io/{domain}/theme/dark/logo?c={BRANDFETCH_CLIENT_ID}"


def seed_raw_data(descriptions: List[str]) -> None:
    if not descriptions:
        print("No transactions to seed.")
        return
    rows = [{"description": d} for d in descriptions]
    try:
        supabase.table("raw_transactions").upsert(rows, on_conflict="description").execute()
        print(f"Seeded {len(rows)} rows into raw_transactions.")
    except Exception as e:
        print(f"Seeding error (check tables/unique constraint): {e}")


def _pick_preferred_hit(results: List[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    """Prefer domains ending with the configured TLD, else fall back to the first hit."""
    if not results:
        return None
    if COUNTRY_TLD_PREFERENCE:
        for hit in results:
            domain = hit.get("domain") or ""
            if domain.endswith(COUNTRY_TLD_PREFERENCE):
                return hit
    return results[0]


def search_brand(description: str) -> Optional[Dict[str, Any]]:
    """Brand Search API (name -> list of candidates)."""
    url = f"https://api.brandfetch.io/v2/search/{requests.utils.quote(description)}"
    params = {"c": BRANDFETCH_CLIENT_ID}
    try:
        resp = requests.get(url, params=params, timeout=10)
        if resp.status_code == 200:
            data = resp.json()
            if isinstance(data, list) and data:
                return _pick_preferred_hit(data)
        return None
    except Exception as exc:
        print(f"  Brandfetch search error for '{description}': {exc}")
        return None


def fetch_brand_profile(domain: str) -> Optional[Dict[str, Any]]:
    """Brand API: fetch full profile by domain."""
    url = f"https://api.brandfetch.io/v2/brands/{domain}"
    headers = {"Authorization": f"Bearer {BRANDFETCH_API_KEY}"}
    try:
        resp = requests.get(url, headers=headers, timeout=10)
        if resp.status_code == 200:
            return resp.json()
        return None
    except Exception as exc:
        print(f"  Brandfetch profile error for '{domain}': {exc}")
        return None


def enrich_transactions(descriptions: List[str]) -> None:
    if not descriptions:
        print("No transactions to enrich.")
        return
    try:
        res = (
            supabase.table("raw_transactions")
            .select("*")
            .in_("description", descriptions)
            .eq("processed", False)
            .execute()
        )
        transactions = res.data
    except Exception as e:
        print(f"Error fetching raw transactions (ensure tables exist): {e}")
        return

    if not transactions:
        print("Nothing to process (all provided lines already processed).")
        return

    matches, misses = 0, 0
    for tx in transactions:
        desc = tx["description"]
        tx_id = tx["id"]
        print(f"Processing: {desc}")

        # Step 1: search to get a candidate domain (prefers .au)
        search_hit = search_brand(desc)
        domain = search_hit.get("domain") if search_hit else None

        # Step 2: Brand API lookup to get richer profile (name, company/location, quality)
        profile = fetch_brand_profile(domain) if domain else None

        if profile or search_hit:
            chosen = profile or search_hit
            domain = chosen.get("domain") or domain
            company = chosen.get("company") or {}
            location = company.get("location") or {}
            enriched_data = {
                "transaction_cache": desc,
                "brand_name": chosen.get("name"),
                "website_url": f"https://{domain}" if domain else None,
                "logo": get_logo_url(domain),
                "confidence_score": chosen.get("qualityScore", 0),
                "brandfetch_id": chosen.get("id"),
                "country_code": location.get("countryCode"),
                "company_country": location.get("country"),
                "company_city": location.get("city"),
                "full_response": profile or search_hit,
            }
            matches += 1
        else:
            enriched_data = {
                "transaction_cache": desc,
                "confidence_score": 0.0,
            }
            misses += 1

        try:
            supabase.table("enriched_merchants").upsert(enriched_data, on_conflict="transaction_cache").execute()
            supabase.table("raw_transactions").update({"processed": True}).eq("id", tx_id).execute()
            print("  Stored.")
        except Exception as e:
            print(f"  Storage error: {e}")

    # Clean up rows with NULL brand_name or NULL website_url
    try:
        supabase.table("enriched_merchants").delete().or_("brand_name.is.null,website_url.is.null").execute()
    except Exception as e:
        print(f"Cleanup error: {e}")

    print("\n--- Summary ---")
    print(f"Matched: {matches}")
    print(f"No match: {misses}")
    print("Check Supabase -> Table Editor -> enriched_merchants for results.")


if __name__ == "__main__":
    tx_lines = load_transactions()
    seed_raw_data(tx_lines)
    enrich_transactions(tx_lines)
