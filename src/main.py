"""Main orchestration - Complete verification pipeline
Architecture:
1. ABN Lookup - Get ABN, legal name, state from ABR
2. Google Verification - Verify legal name & ABN are correct
3. Address Lookup - Find head office address
4. Output - Save enriched merchant data with verification confidence
"""
import os
import json
import warnings
from pathlib import Path
from dotenv import load_dotenv

from abr_client import ABRClient
from google_search_client import GoogleSearchClient
from data_processor import DataProcessor

# Load environment variables from .env
load_dotenv()

# Suppress warnings
warnings.filterwarnings("ignore", message="urllib3")


def main():
    # Load configuration from environment variables
    config = {
        "abr_guid": os.getenv("ABR_GUID"),
        "abr_endpoint": os.getenv("ABR_ENDPOINT"),
        "timeout": int(os.getenv("TIMEOUT", "5")),
        "google_api_key": os.getenv("GOOGLE_API_KEY", ""),
        "google_search_engine_id": os.getenv("GOOGLE_SEARCH_ENGINE_ID", ""),
        "google_client_id": os.getenv("GOOGLE_CLIENT_ID", ""),
        "google_client_secret": os.getenv("GOOGLE_CLIENT_SECRET", ""),
        "google_redirect_uri": os.getenv("GOOGLE_REDIRECT_URI", "http://localhost:8080/callback"),
        "output_file": os.getenv("OUTPUT_FILE", "enriched_merchants_demo.csv"),
        "enable_verification": os.getenv("ENABLE_VERIFICATION", "true").lower() == "true"
    }
    
    # Initialize ABR client
    abr_client = ABRClient(
        guid=config["abr_guid"],
        endpoint=config["abr_endpoint"],
        timeout=config["timeout"]
    )
    
    # Initialize Google Search client (only if credentials present and enabled)
    google_client = None
    if config.get("enable_verification", False):
        api_key = config.get("google_api_key", "")
        search_engine_id = config.get("google_search_engine_id", "")
        client_id = config.get("google_client_id", "")
        client_secret = config.get("google_client_secret", "")
        
        # Check if all Google credentials are properly configured
        if (not api_key.startswith("YOUR_") and 
            not search_engine_id.startswith("YOUR_") and 
            client_id and client_secret):
            try:
                google_client = GoogleSearchClient(
                    api_key=api_key,
                    search_engine_id=search_engine_id,
                    client_id=client_id,
                    client_secret=client_secret,
                    timeout=config["timeout"]
                )
                print("✓ Google Custom Search API initialized\n")
            except Exception as e:
                print(f"⚠️  Google verification disabled: {str(e)}\n")
        else:
            print("⚠️  Google verification disabled: Incomplete credentials\n")
    
    # Initialize data processor
    processor = DataProcessor(config["output_file"])
    
    # Process each merchant
    print(f"Processing {len(config['merchants'])} merchants...\n")
    print("Architecture: ABN Lookup → Google Verification → Address Lookup → Output\n")
    
    for i, merchant in enumerate(config["merchants"], 1):
        # Normalize merchant name
        brand_name = merchant.strip().title()
        
        # STEP 1: ABN Lookup - Get ABN, state, legal name
        abn, state, legal_name, score = abr_client.lookup(brand_name)
        
        # STEP 2: Google Verification (if enabled)
        verification_confidence = 0
        verified = False
        head_office = None
        
        if google_client and abn:
            try:
                # Verify ABN and legal name through Google (with fallback attempts)
                enriched = google_client.verify_and_enrich(abn, legal_name, state)
                verification = enriched.get("verification", {})
                verified = verification.get("verified", False)
                verification_confidence = verification.get("confidence", 0)
                method = verification.get("method", "unknown")
                
                # STEP 3: Get head office address
                head_office = enriched.get("head_office", {})
                address = head_office.get("address") if head_office else None
                
                # Extract what Google found (for correction purposes)
                google_found = enriched.get("google_found", {})
                google_abn = google_found.get("abn", "")
                google_legal_name = google_found.get("legal_name", "")
                
                # Log if Google found different data
                if not verified and (google_abn or google_legal_name):
                    print(f"  (Google found: ABN={google_abn}, Name={google_legal_name})")
                
            except Exception as e:
                print(f"  Verification error: {str(e)}")
                address = None
                google_abn = ""
                google_legal_name = ""
        else:
            address = None
            google_abn = ""
            google_legal_name = ""
        
        # STEP 4: Store enriched result with verification data
        processor.add_result(
            merchant_name=merchant,
            abn=abn,
            state=state,
            legal_name=legal_name,
            score=score,
            verified=verified,
            confidence=verification_confidence,
            address=address,
            google_abn=google_abn,
            google_legal_name=google_legal_name
        )
        
        # Progress indicator
        abn_status = "✓" if abn else "✗"
        verify_status = "✓" if verified else "○" if abn else "—"
        addr_status = "✓" if address else "—"
        
        print(f"[{i:2d}/{len(config['merchants'])}] ABN:{abn_status} Verify:{verify_status} Addr:{addr_status} {merchant:30s}")
        if abn:
            print(f"         → ABN: {abn} | Legal: {legal_name:30s} | Confidence: {verification_confidence:.0f}%")
        if address:
            print(f"         → Address: {address}")
    
    # STEP 5: Save and display results
    output_path = processor.save_to_file()
    processor.print_summary()
    
    print(f"\n✓ Output saved: {output_path.resolve()}")


if __name__ == "__main__":
    main()
