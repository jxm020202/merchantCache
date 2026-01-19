"""Google Custom Search API Client - Handles verification and address lookup"""
import requests
import json
import re
from typing import Dict, List, Optional, Tuple
from urllib.parse import urlencode


class GoogleSearchClient:
    """
    Client for Google Custom Search API to verify ABN/legal details and find head office addresses.
    
    Setup Instructions:
    1. Go to https://console.cloud.google.com/
    2. Create a new project
    3. Enable Custom Search API
    4. Create an API key in Credentials
    5. Setup a Custom Search Engine at https://programmablesearchengine.google.com/
    6. Add config to config.json with:
       - "google_api_key": "YOUR_API_KEY"
       - "google_search_engine_id": "YOUR_SEARCH_ENGINE_ID"
    """
    
    def __init__(self, api_key: str, search_engine_id: str, timeout: int = 5, client_id: str = None, client_secret: str = None):
        """
        Initialize Google Search Client
        
        Args:
            api_key: Google Custom Search API key
            search_engine_id: Custom Search Engine ID
            timeout: Request timeout in seconds
            client_id: Optional OAuth2 client ID (from config)
            client_secret: Optional OAuth2 client secret (from config or env var)
        """
        self.api_key = api_key
        self.search_engine_id = search_engine_id
        self.timeout = timeout
        self.base_url = "https://www.googleapis.com/customsearch/v1"
        self.client_id = client_id
        self.client_secret = client_secret  # Should come from config.json or environment
    
    def get_auth_redirect_url(self, redirect_uri: str = "http://localhost:8080/callback") -> str:
        """
        Get the Google OAuth2 redirect URL for authentication setup.
        
        Args:
            redirect_uri: Redirect URI registered in Google Console
            
        Returns:
            str: OAuth2 redirect URL for authentication
            
        Raises:
            ValueError: If client_id is not configured
        """
        if not self.client_id:
            raise ValueError("client_id not configured. Add 'google_client_id' to config.json")
        
        params = {
            "client_id": self.client_id,
            "redirect_uri": redirect_uri,
            "response_type": "code",
            "scope": "https://www.googleapis.com/auth/cse",
            "access_type": "offline",
            "prompt": "consent"
        }
        return f"https://accounts.google.com/o/oauth2/v2/auth?{urlencode(params)}"
    
    def get_access_token(self, auth_code: str, redirect_uri: str = "http://localhost:8080/callback") -> Optional[Dict]:
        """
        Exchange authorization code for access token.
        
        Args:
            auth_code: Authorization code from OAuth2 redirect
            redirect_uri: Must match registered redirect URI
            
        Returns:
            Dictionary with access_token, refresh_token, etc. or None if failed
        """
        if not self.client_secret:
            raise ValueError("client_secret not configured. Add 'google_client_secret' to config.json")
        
        token_url = "https://oauth2.googleapis.com/token"
        payload = {
            "code": auth_code,
            "client_id": self.client_id,
            "client_secret": self.client_secret,
            "redirect_uri": redirect_uri,
            "grant_type": "authorization_code"
        }
        
        try:
            response = requests.post(token_url, json=payload, timeout=self.timeout)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            print(f"Token exchange failed: {str(e)}")
            return None
    
    def search(self, query: str, num_results: int = 10) -> List[Dict]:
        """
        Perform a Google Custom Search
        
        Args:
            query: Search query string
            num_results: Number of results to return (max 10 per request)
            
        Returns:
            List of search results with title, link, snippet
        """
        params = {
            "q": query,
            "key": self.api_key,
            "cx": self.search_engine_id,
            "num": min(num_results, 10)
        }
        
        try:
            response = requests.get(self.base_url, params=params, timeout=self.timeout)
            response.raise_for_status()
            data = response.json()
            
            if "items" not in data:
                return []
            
            results = []
            for item in data.get("items", []):
                results.append({
                    "title": item.get("title", ""),
                    "link": item.get("link", ""),
                    "snippet": item.get("snippet", "")
                })
            return results
        
        except requests.exceptions.RequestException as e:
            print(f"Search request failed: {str(e)}")
            return []
    
    def verify_abn_details(self, abn: str, legal_name: str, retry: bool = True) -> Dict[str, any]:
        """
        Verify ABN and legal name details using Google Search
        
        Args:
            abn: ABN number (11 digits)
            legal_name: Legal company name
            retry: If True and initial verification fails, try alternative searches
            
        Returns:
            Dictionary with verification status and confidence score
        """
        # Validate ABN format
        abn_clean = re.sub(r'\D', '', abn)
        if len(abn_clean) != 11:
            return {
                "verified": False,
                "confidence": 0,
                "reason": "Invalid ABN format - must be 11 digits"
            }
        
        # ATTEMPT 1: Primary search with ABN + legal name
        query = f"ABN {abn_clean} {legal_name} Australia"
        results = self.search(query, num_results=5)
        
        if results:
            # Calculate confidence based on snippet matches
            abn_matches = sum(1 for r in results if abn_clean in r["snippet"])
            name_matches = sum(1 for r in results if legal_name.lower() in r["snippet"].lower())
            confidence = (abn_matches * 0.6 + name_matches * 0.4) / len(results) * 100
            
            if confidence >= 50:
                return {
                    "verified": True,
                    "confidence": round(confidence, 2),
                    "results_count": len(results),
                    "top_result": results[0] if results else None,
                    "method": "primary"
                }
        
        # ATTEMPT 2: Retry with alternative searches if primary fails
        if retry:
            return self._verify_with_fallback(abn_clean, legal_name)
        
        return {
            "verified": False,
            "confidence": 0,
            "reason": "No search results found",
            "method": "primary"
        }
    
    def _verify_with_fallback(self, abn: str, legal_name: str) -> Dict:
        """
        Fallback verification with alternative search strategies
        
        Args:
            abn: ABN number (clean, 11 digits)
            legal_name: Legal company name
            
        Returns:
            Dictionary with verification status and confidence
        """
        # Extract core company name (remove PTY LTD, LIMITED, etc.)
        core_name = self._extract_core_name(legal_name)
        
        fallback_queries = [
            # Strategy 1: Just company name + ABN
            f"{legal_name} ABN {abn}",
            # Strategy 2: Core name + ABN
            f"{core_name} ABN {abn}",
            # Strategy 3: Company name with Australia
            f"{legal_name} Australia",
            # Strategy 4: Core name only
            f"{core_name}",
            # Strategy 5: ABN lookup
            f"ABN {abn} {core_name}",
        ]
        
        best_result = {
            "verified": False,
            "confidence": 0,
            "reason": "Failed all verification attempts",
            "method": "fallback"
        }
        
        for i, query in enumerate(fallback_queries, 1):
            try:
                results = self.search(query, num_results=5)
                
                if not results:
                    continue
                
                # Advanced scoring: Check for ABN or name in results
                abn_matches = sum(1 for r in results if abn in r["snippet"] or abn in r["title"])
                name_matches = sum(1 for r in results if self._is_name_match(legal_name, r))
                core_matches = sum(1 for r in results if self._is_name_match(core_name, r))
                
                # Calculate confidence with better logic
                if abn_matches > 0:
                    confidence = 85  # Strong: ABN found
                elif name_matches > 0:
                    confidence = 75  # Good: Legal name found
                elif core_matches > 1:
                    confidence = 65  # Fair: Core name found multiple times
                elif core_matches > 0:
                    confidence = 55  # Weak: Core name found once
                else:
                    confidence = 0
                
                if confidence > best_result["confidence"]:
                    best_result = {
                        "verified": confidence >= 50,
                        "confidence": round(confidence, 2),
                        "results_count": len(results),
                        "top_result": results[0] if results else None,
                        "method": f"fallback_strategy_{i}",
                        "query": query
                    }
                    
                    # If we found strong match, return immediately
                    if confidence >= 75:
                        return best_result
            
            except Exception as e:
                continue
        
        return best_result
    
    @staticmethod
    def _extract_core_name(company_name: str) -> str:
        """
        Extract core company name, removing legal suffixes
        
        Args:
            company_name: Full company name
            
        Returns:
            Core name without PTY LTD, LIMITED, etc.
        """
        # Remove common legal suffixes
        suffixes = [
            r'\s+PTY\s+LTD\.?',
            r'\s+PTY\.?',
            r'\s+LIMITED',
            r'\s+LTD\.?',
            r'\s+A\.E\.N\.?',
            r'\s+PARTNERSHIP',
            r'\s+A\.N\.?',
            r'\(.*?\)',  # Remove anything in parentheses
        ]
        
        result = company_name
        for suffix in suffixes:
            result = re.sub(suffix, '', result, flags=re.IGNORECASE)
        
        return result.strip()
    
    @staticmethod
    def _is_name_match(company_name: str, result: Dict) -> bool:
        """
        Check if company name appears in search result
        
        Args:
            company_name: Company name to search for
            result: Search result dict with 'title' and 'snippet'
            
        Returns:
            True if name matches in title or snippet
        """
        text = (result.get("title", "") + " " + result.get("snippet", "")).lower()
        
        # Remove punctuation for matching
        name_clean = re.sub(r'[^a-z0-9\s]', '', company_name.lower())
        
        # Check if any significant portion of the name appears
        name_parts = name_clean.split()
        if len(name_parts) > 2:
            # For long names, check if at least 2 parts match
            matches = sum(1 for part in name_parts if part in text)
            return matches >= 2
        else:
            # For short names, check if exact portion exists
            return name_clean in text
    
    def extract_abn_from_search(self, results: List[Dict]) -> Optional[str]:
        """
        Extract ABN from search results
        
        Args:
            results: List of search results
            
        Returns:
            ABN if found, None otherwise
        """
        abn_pattern = r'\b(\d{2}\s?\d{3}\s?\d{3}\s?\d{3}|\d{11})\b'
        
        for result in results:
            text = result.get("snippet", "") + " " + result.get("title", "")
            matches = re.findall(abn_pattern, text)
            if matches:
                # Clean and return first match
                abn = re.sub(r'\s', '', matches[0])
                if len(abn) == 11:
                    return abn
        return None
    
    def find_correct_legal_name(self, abn: str, merchant_name: str = "") -> Optional[Dict]:
        """
        Find the CORRECT and most official legal name for an ABN
        
        Args:
            abn: ABN number (11 digits)
            merchant_name: Optional original merchant name to help search
            
        Returns:
            Dictionary with found name, confidence, and source URL
        """
        # Clean ABN
        abn_clean = re.sub(r'\D', '', abn)
        if len(abn_clean) != 11:
            return None
        
        # Search strategies to find official name
        queries = [
            f"ABN {abn_clean} company name registration",
            f"ABN {abn_clean} official name",
            f"\"ABN {abn_clean}\" {merchant_name}",
            f"ACN ABN {abn_clean} Australia",
        ]
        
        best_name = None
        best_confidence = 0
        best_url = None
        
        for query in queries:
            try:
                results = self.search(query, num_results=5)
                
                for result in results:
                    title = result.get("title", "")
                    snippet = result.get("snippet", "")
                    url = result.get("link", "")
                    
                    # Look for company name patterns
                    # Usually: "Company Name - Official ABN Registration" or similar
                    patterns = [
                        r'^([A-Za-z0-9\s\-&\.\']+?)\s*(?:[-|•]|ABN|ACN)',
                        r'([A-Za-z0-9\s\-&\.\']+?)\s+(?:ABN|ACN)\s+' + abn_clean,
                        r'(?:Name|Company|Legal):\s*([A-Za-z0-9\s\-&\.\']+?)(?:\s+[,|•]|$)',
                    ]
                    
                    for pattern in patterns:
                        match = re.search(pattern, title + " " + snippet, re.IGNORECASE)
                        if match:
                            found_name = match.group(1).strip()
                            
                            # Filter out short or irrelevant names
                            if len(found_name) > 5 and "click" not in found_name.lower():
                                # Confidence based on where it appeared
                                confidence = 90 if abn_clean in snippet else 80
                                
                                if confidence > best_confidence:
                                    best_confidence = confidence
                                    best_name = found_name
                                    best_url = url
            
            except Exception:
                continue
        
        if best_name:
            return {
                "legal_name": best_name,
                "confidence": best_confidence,
                "source_url": best_url,
                "method": "abn_lookup"
            }
        
        return None
        """
        Extract company legal name from search results
        
        Args:
            results: List of search results
            merchant_name: Original merchant name
            
        Returns:
            Extracted legal name if found, None otherwise
        """
        # Look for company names in titles first (usually more reliable)
        for result in results:
            title = result.get("title", "")
            snippet = result.get("snippet", "")
            
            # Common patterns: "Company Name | ..." or "Company Name - ..."
            match = re.search(r'^([A-Za-z\s&\-\.\']+?)(?:\s*[|\-]|$)', title)
            if match:
                name = match.group(1).strip()
                if len(name) > 3 and name.lower() != merchant_name.lower():
                    return name
        
        return None
    
    def find_head_office_address(self, legal_name: str, state: str = "") -> Optional[Dict]:
        """
        Find head office address for a company
        
        Args:
            legal_name: Legal company name
            state: Optional Australian state code (NSW, VIC, QLD, etc.)
            
        Returns:
            Dictionary with address details or None if not found
        """
        # Build search query
        state_query = f"{state}" if state else "Australia"
        query = f"{legal_name} head office address {state_query}"
        
        results = self.search(query, num_results=10)
        
        if not results:
            return None
        
        # Parse address from top results
        for result in results:
            address = self._extract_address(result["snippet"], result["title"])
            if address:
                return {
                    "address": address,
                    "source_title": result["title"],
                    "source_url": result["link"],
                    "snippet": result["snippet"]
                }
        
        return None
    
    @staticmethod
    def _extract_address(snippet: str, title: str) -> Optional[str]:
        """
        Extract address from search snippet and title
        
        Args:
            snippet: Search result snippet
            title: Search result title
            
        Returns:
            Extracted address or None
        """
        # Common Australian address patterns
        patterns = [
            r"(?:Address|Headquarters?|Office)?:?\s*(\d+\s+[A-Za-z\s]+(?:Street|Street|Rd|Road|Ave|Avenue|Blvd|Boulevard|Court|Ct|Drive|Dr|Way|Lane|Ln|Close|Cl)[\w\s]*(?:,?\s*[A-Z]{2}\s+\d{4})?)",
            r"([A-Za-z0-9\s]+(?:Street|Rd|Road|Ave|Avenue|Drive|Dr|Lane|Ln|Court|Ct|Way|Close|Cl).*?[A-Z]{2}\s+\d{4})"
        ]
        
        combined_text = f"{title} {snippet}"
        
        for pattern in patterns:
            match = re.search(pattern, combined_text, re.IGNORECASE)
            if match:
                return match.group(1).strip()
        
        return None
        """
        Find head office address for a company
        
        Args:
            legal_name: Legal company name
            state: Optional Australian state code (NSW, VIC, QLD, etc.)
            
        Returns:
            Dictionary with address details or None if not found
        """
        # Build search query
        state_query = f"{state}" if state else "Australia"
        query = f"{legal_name} head office address {state_query}"
        
        results = self.search(query, num_results=10)
        
        if not results:
            return None
        
        # Parse address from top results
        for result in results:
            address = self._extract_address(result["snippet"], result["title"])
            if address:
                return {
                    "address": address,
                    "source_title": result["title"],
                    "source_url": result["link"],
                    "snippet": result["snippet"]
                }
        
        return None
    
    @staticmethod
    def _extract_address(snippet: str, title: str) -> Optional[str]:
        """
        Extract address from search snippet and title
        
        Args:
            snippet: Search result snippet
            title: Search result title
            
        Returns:
            Extracted address or None
        """
        # Common Australian address patterns
        patterns = [
            r"(?:Address|Headquarters?|Office)?:?\s*(\d+\s+[A-Za-z\s]+(?:Street|Street|Rd|Road|Ave|Avenue|Blvd|Boulevard|Court|Ct|Drive|Dr|Way|Lane|Ln|Close|Cl)[\w\s]*(?:,?\s*[A-Z]{2}\s+\d{4})?)",
            r"([A-Za-z0-9\s]+(?:Street|Rd|Road|Ave|Avenue|Drive|Dr|Lane|Ln|Court|Ct|Way|Close|Cl).*?[A-Z]{2}\s+\d{4})"
        ]
        
        combined_text = f"{title} {snippet}"
        
        for pattern in patterns:
            match = re.search(pattern, combined_text, re.IGNORECASE)
            if match:
                return match.group(1).strip()
        
        return None
    
    def verify_and_enrich(self, abn: str, legal_name: str, state: str = "") -> Dict:
        """
        Complete verification and enrichment process
        
        Args:
            abn: ABN number
            legal_name: Legal company name
            state: Optional Australian state
            
        Returns:
            Dictionary with verification and address details, plus Google-found data
        """
        # Verify ABN and legal details
        verification = self.verify_abn_details(abn, legal_name)
        
        # Find head office address
        address_info = None
        if verification["verified"]:
            address_info = self.find_head_office_address(legal_name, state)
        
        # Extract what Google found
        google_abn = None
        google_legal_name = None
        
        if verification.get("top_result"):
            # Try to extract ABN from search results
            top_result = verification.get("top_result", {})
            results_for_extraction = [top_result]
            google_abn = self.extract_abn_from_search(results_for_extraction)
        
        return {
            "abn": abn,
            "legal_name": legal_name,
            "state": state,
            "verification": verification,
            "head_office": address_info,
            "google_found": {
                "abn": google_abn,
                "legal_name": google_legal_name
            }
        }


# Setup guide for authentication
SETUP_INSTRUCTIONS = """
=== GOOGLE CUSTOM SEARCH API SETUP ===

1. CREATE API KEY:
   - Go to: https://console.cloud.google.com/
   - Create new project
   - Enable "Custom Search API"
   - Go to Credentials → Create API Key
   - Copy the API key

2. CREATE CUSTOM SEARCH ENGINE:
   - Go to: https://programmablesearchengine.google.com/
   - Create new search engine (search the entire web)
   - Copy the Search Engine ID (cx)

3. UPDATE config.json:
   Add these fields:
   {
     "google_api_key": "YOUR_API_KEY_HERE",
     "google_search_engine_id": "YOUR_SEARCH_ENGINE_ID_HERE"
   }

4. OPTIONAL - OAUTH2 AUTHENTICATION:
   For authenticated requests, get redirect URL:
   from google_search_client import GoogleSearchClient
   print(GoogleSearchClient.get_auth_redirect_url())
   
   Then use the auth code to get access token for higher rate limits.

5. RATE LIMITS:
   - Free tier: 100 queries/day
   - Paid tier: Up to 10,000 queries/day
"""
