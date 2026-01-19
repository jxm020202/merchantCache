"""Test Google Search Client Setup"""
import json
import sys
from pathlib import Path

# Add parent directory to path to import modules
sys.path.insert(0, str(Path(__file__).parent.parent))

from google_search_client import GoogleSearchClient


def test_google_search_client():
    """Test Google Search Client with configuration"""
    
    # Load config from parent directory
    config_path = Path(__file__).parent.parent / "config.json"
    with open(config_path, "r") as f:
        config = json.load(f)
    
    # Check if credentials are complete
    print("=" * 60)
    print("GOOGLE CUSTOM SEARCH API - TEST SETUP")
    print("=" * 60)
    
    # Validate credentials
    required_fields = {
        "google_api_key": "API Key",
        "google_search_engine_id": "Search Engine ID",
        "google_client_id": "Client ID"
    }
    
    missing = []
    for field, display_name in required_fields.items():
        value = config.get(field, "")
        if value.startswith("YOUR_") or not value:
            missing.append(display_name)
            print(f"❌ {display_name}: MISSING")
        else:
            print(f"✓ {display_name}: {'*' * 10}...{value[-10:]}")
    
    client_secret = config.get("google_client_secret", "")
    if client_secret:
        print(f"✓ Client Secret: {'*' * 10}...{client_secret[-10:]}")
    else:
        print(f"❌ Client Secret: MISSING")
        missing.append("Client Secret")
    
    print()
    
    if missing:
        print(f"⚠️  MISSING: {', '.join(missing)}")
        print("\nTo complete setup:")
        print("1. Go to: https://console.cloud.google.com/")
        print("2. Get your API Key and Client ID")
        print("3. Go to: https://programmablesearchengine.google.com/")
        print("4. Get your Search Engine ID")
        print("5. Update config.json with these values")
        return False
    
    print("✅ All credentials present! Testing API calls...")
    print()
    
    try:
        # Initialize client
        client = GoogleSearchClient(
            api_key=config["google_api_key"],
            search_engine_id=config["google_search_engine_id"],
            client_id=config["google_client_id"],
            client_secret=config["google_client_secret"]
        )
        
        # Test 1: Get redirect URL
        print("TEST 1: OAuth2 Redirect URL")
        print("-" * 60)
        try:
            redirect_url = client.get_auth_redirect_url(
                redirect_uri=config.get("google_redirect_uri", "http://localhost:8080/callback")
            )
            print(f"✓ Generated redirect URL:")
            print(f"  {redirect_url[:80]}...")
            print()
        except Exception as e:
            print(f"❌ Error: {str(e)}")
            print()
        
        # Test 2: Search function
        print("TEST 2: Search Function")
        print("-" * 60)
        try:
            results = client.search("Woolworths Australia head office", num_results=3)
            if results:
                print(f"✓ Found {len(results)} search results:")
                for i, result in enumerate(results, 1):
                    print(f"  {i}. {result['title']}")
                    print(f"     {result['snippet'][:70]}...")
            else:
                print("❌ No results returned (check API key and search engine ID)")
            print()
        except Exception as e:
            print(f"❌ Search error: {str(e)}")
            print()
        
        # Test 3: ABN verification
        print("TEST 3: ABN Verification")
        print("-" * 60)
        try:
            result = client.verify_abn_details("88000014675", "Woolworths Group Limited")
            print(f"✓ Verification result:")
            print(f"  Verified: {result['verified']}")
            print(f"  Confidence: {result['confidence']}%")
            print(f"  Reason: {result.get('reason', 'N/A')}")
            print()
        except Exception as e:
            print(f"❌ Verification error: {str(e)}")
            print()
        
        # Test 4: Address lookup
        print("TEST 4: Head Office Address Lookup")
        print("-" * 60)
        try:
            address = client.find_head_office_address("Woolworths Group Limited", "NSW")
            if address:
                print(f"✓ Found address:")
                print(f"  {address['address']}")
                print(f"  Source: {address['source_title']}")
            else:
                print("❌ No address found")
            print()
        except Exception as e:
            print(f"❌ Address lookup error: {str(e)}")
            print()
        
        print("=" * 60)
        print("✅ TESTS COMPLETE")
        print("=" * 60)
        
    except Exception as e:
        print(f"❌ Error initializing client: {str(e)}")
        return False
    
    return True


if __name__ == "__main__":
    success = test_google_search_client()
    exit(0 if success else 1)
