"""Main orchestration - Coordinates ABR lookup and data processing"""
import json
import warnings
import sys
from pathlib import Path

# Add parent directory to path to import modules
sys.path.insert(0, str(Path(__file__).parent.parent))

from abr_client import ABRClient
from data_processor import DataProcessor

# Suppress warnings
warnings.filterwarnings("ignore", message="urllib3")


def main():
    # Load configuration from parent directory
    config_path = Path(__file__).parent.parent / "config.json"
    with open(config_path, "r") as f:
        config = json.load(f)
    
    # Initialize ABR client
    abr_client = ABRClient(
        guid=config["abr_guid"],
        endpoint=config["abr_endpoint"],
        timeout=config["timeout"]
    )
    
    # Initialize data processor
    processor = DataProcessor(config["output_file"])
    
    # Process each merchant
    print(f"Processing {len(config['merchants'])} merchants...\n")
    
    for i, merchant in enumerate(config["merchants"], 1):
        # Normalize merchant name
        brand_name = merchant.strip().title()
        
        # Lookup ABN, state, legal name, and score
        abn, state, legal_name, score = abr_client.lookup(brand_name)
        
        # Store result
        processor.add_result(merchant, abn, state, legal_name, score)
        
        # Progress indicator
        status = "✓" if abn else "✗"
        print(f"[{i:2d}/{len(config['merchants'])}] {status} {merchant:30s} -> ABN: {abn:15s} State: {state:5s} Score: {score:3s}")
    
    # Save and display results
    output_path = processor.save_to_file()
    processor.print_summary()
    
    print(f"Wrote: {output_path.resolve()}")


if __name__ == "__main__":
    main()


# ================== TROUBLESHOOTING ==================
# Issue: ModuleNotFoundError for abr_client or data_processor
# Solution: Run from correct directory. Files must be in same folder as main.py
#
# Issue: config.json Not Found
# Solution: Ensure config.json is in /Users/intern/Desktop/abntest/
#
# Issue: Invalid JSON Error
# Solution: Check JSON syntax at https://jsonlint.com/ - no trailing commas
#
# Issue: GUID Invalid
# Solution: Register at https://abr.business.gov.au/Tools/WebServices
#
# Issue: All Merchants Return Empty
# Solution: Test with known merchant like 'Woolworths'. Check API status.
#
# Issue: Script Hangs
# Solution: Press Ctrl+C. Increase timeout in config.json.
#
# Issue: No Output While Running
# Solution: Run with: python -u main.py (unbuffered output)
#
# Directory structure required:
# abntest/
#   ├── main.py
#   ├── abr_client.py
#   ├── data_processor.py
#   ├── config.json
#   └── .venv/
