# ABN Lookup Architecture Documentation

## Overview
This script performs batch ABN (Australian Business Number) lookups for a list of merchant names using the Australian Business Register (ABR) web services API.

## System Architecture

### 1. Input
- **MERCHANTS List**: A predefined list of merchant names (strings)
- Example: `["kmart", "paypal", "bunnings", "apple", ...]`

### 2. Processing Flow

```
┌─────────────────────────────────────────────────────────────┐
│ For each merchant name in MERCHANTS list                    │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 1. Normalize merchant name (Title case)                      │
│    Example: "jb hi fi" → "Jb Hi Fi"                         │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Call ABR API (ABRSearchByNameSimpleProtocol)             │
│    - Endpoint: https://abr.business.gov.au/...             │
│    - Parameters: name, authenticationGuid                   │
│    - Returns: XML response                                  │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Parse XML Response                                       │
│    - Extract ABN (11 digits)                                │
│    - Extract Business Name                                  │
│    - Extract Score (0-100)                                  │
│    - Verify Active status (Y/N)                             │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Select Best Candidate                                    │
│    - Filter: Only active ABNs (isCurrent = "Y")            │
│    - Filter: Valid 11-digit ABN format                      │
│    - Select: Highest score match                            │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. Store Result                                             │
│    Row: {merchant, abn}                                     │
│    Example: {"merchant": "apple", "abn": "50169260144"}    │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 6. Output to CSV                                            │
│    File: enriched_merchants_demo.csv                        │
└─────────────────────────────────────────────────────────────┘
```

## Key Components

### Configuration
```python
DEMO_MODE = False                    # Use real ABR API (not mock)
ABR_GUID = "b5e92e10-b051-..."      # Authentication GUID (required for API)
MERCHANTS = [...]                    # List of merchant names to lookup
```

### API Details

**Endpoint**: `https://abr.business.gov.au/AbrXmlSearch.asmx/ABRSearchByNameSimpleProtocol`

**HTTP Method**: GET

**Parameters**:
- `name` (required): Merchant/business name to search
- `authenticationGuid` (required): Your ABR API authentication key
- Optional filters: postcode, legalName, tradingName, state filters (NSW, VIC, etc.)

**Response Format**: XML with structure:
```xml
<ABRPayloadSearchResults>
  <response>
    <usageStatement>...</usageStatement>
    <dateRegisterLastUpdated>...</dateRegisterLastUpdated>
    <dateTimeRetrieved>...</dateTimeRetrieved>
  </response>
  <searchResultsList>
    <searchResultsRecord>
      <identifierValue>50169260144</identifierValue>          <!-- ABN -->
      <mainName>Apple Pty Ltd</mainName>                      <!-- Business Name -->
      <score>98</score>                                       <!-- Confidence Score (0-100) -->
      <isCurrent>Y</isCurrent>                                <!-- Active Status (Y/N) -->
    </searchResultsRecord>
    <searchResultsRecord>...</searchResultsRecord>
  </searchResultsList>
</ABRPayloadSearchResults>
```

### Data Flow - XML to Output

```
XML Response
    ↓
parse_candidates()
    ├─ Extract all <searchResultsRecord> elements
    ├─ Filter: ABN must be 11 digits (regex: \d{11})
    ├─ Filter: isCurrent must equal "Y" (active)
    └─ Return list of valid candidates: [{abn, name, score}, ...]
    ↓
pick_best_candidate()
    ├─ Sort candidates by score (highest first)
    ├─ Select first (best) result
    └─ Return: (abn, confidence, needs_review, source)
    ↓
Build Row
    └─ {"merchant": raw_name, "abn": abn_value}
    ↓
pandas.DataFrame.to_csv()
    └─ Output file: enriched_merchants_demo.csv
```

## Error Handling

1. **API 404 Errors**: Merchant not found in ABR
   - Silently handled (no error message shown)
   - Row added with empty ABN value

2. **Network Errors**: Connection timeout or request failure
   - 5-second timeout configured
   - Exception caught, empty response returned
   - Row added with empty ABN value

3. **XML Parsing Errors**: Malformed XML response
   - Try-except wrapper catches parsing failures
   - Empty candidate list returned
   - Row added with empty ABN value

## Output Format

**File**: `enriched_merchants_demo.csv`

**Columns**:
- `merchant`: Original merchant name from input list
- `abn`: Australian Business Number (11 digits) or empty if not found

**Example Output**:
```csv
merchant,abn
kmart,
paypal,
bunnings,
apple,50169260144
spotify,50045137103
uber,67110813107
netflix,
```

## Key Functions

### `real_abr_search_by_name(brand_name: str) -> str`
- Makes HTTP GET request to ABR API
- Parameters: merchant name + GUID
- Returns: XML string response

### `parse_candidates(xml_text: str) -> list`
- Parses XML response
- Validates ABN format (11 digits)
- Filters for active records (isCurrent = "Y")
- Returns: List of valid candidate dictionaries

### `pick_best_candidate(candidates: list) -> tuple`
- Sorts candidates by confidence score (descending)
- Selects highest scoring match
- Returns: (abn, confidence, needs_review, source)

## Performance Notes

- **Rate Limiting**: ABR API has reasonable rate limits (check documentation)
- **Timeout**: 5 seconds per request (configurable)
- **Batch Processing**: Script processes merchants sequentially
- **CSV Output**: Pandas handles efficient writing

## Authentication

Your ABR API GUID is stored in:
```python
ABR_GUID = "b5e92e10-b051-4d63-b503-f164dbd56154"
```

**To get a GUID**:
1. Visit: https://abr.business.gov.au/Tools/WebServices
2. Register for access (free)
3. Accept terms and complete registration
4. Receive GUID via email

## Limitations

- Only searches by business name (not ABN lookups)
- Returns maximum 200 results per search (default)
- Some merchants may not be registered with ABR (e.g., foreign companies)
- Matches based on exact name matching with fuzzy logic scoring

## Future Enhancements

- Add postcode filtering for more accurate matches
- Implement caching to avoid duplicate API calls
- Add batch retry logic for failed lookups
- Support filtering by state or business type
- Add score threshold configuration
