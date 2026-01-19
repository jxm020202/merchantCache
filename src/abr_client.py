"""ABR API Client - Handles all ABR lookup requests"""
import requests
import xml.etree.ElementTree as ET
import re


class ABRClient:
    def __init__(self, guid: str, endpoint: str, timeout: int = 5):
        self.guid = guid
        self.endpoint = endpoint
        self.timeout = timeout

    def search_by_name(self, business_name: str) -> str:
        """Search ABR by business name, return raw XML response"""
        params = {
            "name": business_name,
            "postcode": "",
            "legalName": "Y",
            "tradingName": "Y",
            "NSW": "Y",
            "VIC": "Y",
            "QLD": "Y",
            "WA": "Y",
            "SA": "Y",
            "NT": "Y",
            "ACT": "Y",
            "TAS": "Y",
            "authenticationGuid": self.guid
        }
        try:
            response = requests.get(self.endpoint, params=params, timeout=self.timeout)
            response.raise_for_status()
            return response.text
        except Exception as e:
            return ""

    def parse_first_abn(self, xml_text: str) -> str:
        """Extract first active ABN from XML response"""
        if not xml_text:
            return ""
        
        try:
            root = ET.fromstring(xml_text)
            ns = {"abr": "http://abr.business.gov.au/ABRXMLSearch/"}
            
            # Find first active record
            for rec in root.findall(".//abr:searchResultsRecord", ns):
                abn_elem = rec.find("abr:ABN/abr:identifierValue", ns)
                abn_status = rec.find("abr:ABN/abr:identifierStatus", ns)
                
                abn = (abn_elem.text or "").strip() if abn_elem is not None else ""
                status = (abn_status.text or "").strip() if abn_status is not None else ""
                
                # Return first active ABN with valid format
                if re.fullmatch(r"\d{11}", abn) and status == "Active":
                    return abn
            
            return ""
        except:
            return ""

    def parse_first_acn(self, xml_text: str) -> str:
        """Extract ACN from XML response - not available in current API"""
        return ""

    def parse_first_state(self, xml_text: str) -> str:
        """Extract state from first active ABN record"""
        if not xml_text:
            return ""
        
        try:
            root = ET.fromstring(xml_text)
            ns = {"abr": "http://abr.business.gov.au/ABRXMLSearch/"}
            
            # Find first active record
            for rec in root.findall(".//abr:searchResultsRecord", ns):
                abn_elem = rec.find("abr:ABN/abr:identifierValue", ns)
                abn_status = rec.find("abr:ABN/abr:identifierStatus", ns)
                
                abn = (abn_elem.text or "").strip() if abn_elem is not None else ""
                status = (abn_status.text or "").strip() if abn_status is not None else ""
                
                # Return state for first active ABN
                if re.fullmatch(r"\d{11}", abn) and status == "Active":
                    state_elem = rec.find("abr:mainBusinessPhysicalAddress/abr:stateCode", ns)
                    state = (state_elem.text or "").strip() if state_elem is not None else ""
                    return state
            
            return ""
        except:
            return ""

    def parse_first_legal_name(self, xml_text: str) -> str:
        """Extract legal name from first active ABN record"""
        if not xml_text:
            return ""
        
        try:
            root = ET.fromstring(xml_text)
            ns = {"abr": "http://abr.business.gov.au/ABRXMLSearch/"}
            
            # Find first active record
            for rec in root.findall(".//abr:searchResultsRecord", ns):
                abn_elem = rec.find("abr:ABN/abr:identifierValue", ns)
                abn_status = rec.find("abr:ABN/abr:identifierStatus", ns)
                
                abn = (abn_elem.text or "").strip() if abn_elem is not None else ""
                status = (abn_status.text or "").strip() if abn_status is not None else ""
                
                # Return legal name for first active ABN
                if re.fullmatch(r"\d{11}", abn) and status == "Active":
                    # Try mainTradingName first
                    name_elem = rec.find("abr:mainTradingName/abr:organisationName", ns)
                    if name_elem is not None:
                        return (name_elem.text or "").strip()
                    
                    # Fallback to mainName
                    name_elem = rec.find("abr:mainName/abr:organisationName", ns)
                    if name_elem is not None:
                        return (name_elem.text or "").strip()
                    
                    return ""
            
            return ""
        except:
            return ""

    def parse_first_score(self, xml_text: str) -> str:
        """Extract confidence score from first active ABN record"""
        if not xml_text:
            return ""
        
        try:
            root = ET.fromstring(xml_text)
            ns = {"abr": "http://abr.business.gov.au/ABRXMLSearch/"}
            
            # Find first active record and get score from mainTradingName
            for rec in root.findall(".//abr:searchResultsRecord", ns):
                abn_elem = rec.find("abr:ABN/abr:identifierValue", ns)
                abn_status = rec.find("abr:ABN/abr:identifierStatus", ns)
                
                abn = (abn_elem.text or "").strip() if abn_elem is not None else ""
                status = (abn_status.text or "").strip() if abn_status is not None else ""
                
                # Return score for first active ABN
                if re.fullmatch(r"\d{11}", abn) and status == "Active":
                    # Score is in mainTradingName/score
                    score_elem = rec.find("abr:mainTradingName/abr:score", ns)
                    if score_elem is not None and score_elem.text:
                        return (score_elem.text or "").strip()
                    return ""
            
            return ""
        except:
            return ""

    def get_all_results(self, xml_text: str) -> list:
        """Extract all active results from XML response"""
        if not xml_text:
            return []
        
        results = []
        try:
            root = ET.fromstring(xml_text)
            ns = {"abr": "http://abr.business.gov.au/ABRXMLSearch/"}
            
            for rec in root.findall(".//abr:searchResultsRecord", ns):
                abn_elem = rec.find("abr:ABN/abr:identifierValue", ns)
                abn_status = rec.find("abr:ABN/abr:identifierStatus", ns)
                
                abn = (abn_elem.text or "").strip() if abn_elem is not None else ""
                status = (abn_status.text or "").strip() if abn_status is not None else ""
                
                if re.fullmatch(r"\d{11}", abn) and status == "Active":
                    # Get all fields for this result
                    state_elem = rec.find("abr:mainBusinessPhysicalAddress/abr:stateCode", ns)
                    state = (state_elem.text or "").strip() if state_elem is not None else ""
                    
                    # Extract legal name from businessName (the actual registered name)
                    name_elem = rec.find("abr:businessName/abr:organisationName", ns)
                    legal_name = (name_elem.text or "").strip() if name_elem is not None else ""
                    
                    # Fallback to mainName if businessName is empty
                    if not legal_name:
                        name_elem = rec.find("abr:mainName/abr:organisationName", ns)
                        legal_name = (name_elem.text or "").strip() if name_elem is not None else ""
                    
                    # Final fallback to mainTradingName
                    if not legal_name:
                        name_elem = rec.find("abr:mainTradingName/abr:organisationName", ns)
                        legal_name = (name_elem.text or "").strip() if name_elem is not None else ""
                    
                    score_elem = rec.find("abr:businessName/abr:score", ns)
                    score = (score_elem.text or "").strip() if score_elem is not None else ""
                    
                    results.append({
                        "abn": abn,
                        "state": state,
                        "legal_name": legal_name,
                        "score": score
                    })
        except:
            pass
        
        return results

    def find_best_result(self, business_name: str, results: list) -> dict:
        """Find best matching result using name similarity with strict entity type filtering
        
        Rules:
        1. MUST be active company (PTY LTD, LIMITED, etc)
        2. MUST have at least one primary keyword match
        3. MUST NOT be an unrelated business type (e.g., "cleaning" when searching "spotify")
        4. Penalize results with extra words that indicate different business
        """
        if not results:
            return {"abn": "", "state": "", "legal_name": "", "score": ""}
        
        # Normalize search name
        search_lower = business_name.lower().strip()
        search_words = set(search_lower.split())
        
        # Company type keywords - must have one of these
        company_keywords = ["pty", "limited", "ltd", "inc", "corporation", "corp", "group", "holding"]
        
        # Unrelated business indicators - if present with little match, likely wrong entity
        unrelated_keywords = ["cleaning", "freight", "toners", "candles", "music", "ads", "dogwash", "chill", 
                            "management", "co food", "co-working", "services"]
        
        scored_results = []
        for result in results:
            name_lower = result["legal_name"].lower()
            result_words = set(name_lower.split())
            
            # MUST be company entity
            is_company = any(keyword in name_lower for keyword in company_keywords)
            if not is_company:
                continue
            
            # Check for common words
            common_words = search_words & result_words
            if not common_words:
                # No common words = no match
                continue
            
            # Check if it's an obviously unrelated business type
            # (e.g., "cleaning" when searching for a retail store)
            has_unrelated = any(keyword in name_lower for keyword in unrelated_keywords)
            if has_unrelated and len(common_words) < 2:
                # Unrelated business with only 1 keyword match = skip
                continue
            
            # Calculate match quality
            score_value = int(result["score"]) if result["score"].isdigit() else 50
            
            exact_match = 1000 if search_lower == name_lower else 0
            contains_match = 500 if search_lower in name_lower or name_lower in search_lower else 0
            word_match = 100 * len(common_words)
            
            total_score = exact_match + contains_match + word_match + score_value
            scored_results.append((total_score, result))
        
        # Return best match or empty
        if scored_results:
            return sorted(scored_results, reverse=True, key=lambda x: x[0])[0][1]
        
        return {"abn": "", "state": "", "legal_name": "", "score": ""}

    def lookup(self, business_name: str, verbose: bool = False) -> tuple:
        """Two-stage lookup for improved accuracy
        
        Stage 1: Search by business name, get best match
        Stage 2: Search again using the legal name found in Stage 1 to verify/refine
        
        Returns: (ABN, State, Legal Name, Score) tuple
        """
        # Stage 1: Initial search by business name
        xml_response = self.search_by_name(business_name)
        all_results = self.get_all_results(xml_response)
        best_result = self.find_best_result(business_name, all_results)
        
        if verbose:
            print(f"  [Stage 1] {business_name} -> ABN: {best_result['abn']}, Legal: {best_result['legal_name']}")
        
        # If no result found in stage 1, return empty
        if not best_result["abn"]:
            return (best_result["abn"], best_result["state"], best_result["legal_name"], best_result["score"])
        
        # Stage 2: Secondary search using the legal name for verification
        if best_result["legal_name"]:
            xml_response_2 = self.search_by_name(best_result["legal_name"])
            all_results_2 = self.get_all_results(xml_response_2)
            
            # Look for exact ABN match to confirm validity
            confirmed_result = None
            for result in all_results_2:
                if result["abn"] == best_result["abn"]:
                    confirmed_result = result
                    break
            
            # If confirmed, use stage 2 result (may have better score/name)
            if confirmed_result:
                best_result = confirmed_result
                if verbose:
                    print(f"  [Stage 2] Verified: ABN {best_result['abn']} is valid")
            else:
                # ABN not found in secondary search - might be stale, use stage 1 result
                if verbose:
                    print(f"  [Stage 2] Warning: ABN {best_result['abn']} not confirmed in secondary search")
        
        return (best_result["abn"], best_result["state"], best_result["legal_name"], best_result["score"])


# ================== TROUBLESHOOTING ==================
# Issue: API Returns 404
# Solution: Check endpoint URL is correct
#
# Issue: API Returns 500 with "Missing parameter"
# Solution: Ensure all state codes and parameters are included
#
# Issue: No ABN Results
# Solution: Business may not exist in ABR. Verify at https://abr.business.gov.au/
#
# Issue: Authentication Errors
# Solution: Register GUID at https://abr.business.gov.au/Tools/WebServices
#
# Issue: Connection Timeout
# Solution: Increase timeout value in .env file
