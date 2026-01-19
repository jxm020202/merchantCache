"""Data Processing - Handles data transformation and output"""
import pandas as pd
from pathlib import Path
from typing import List, Dict


class DataProcessor:
    def __init__(self, output_file: str):
        self.output_file = output_file
        self.rows = []

    def add_result(self, merchant_name: str, abn: str = "", state: str = "", legal_name: str = "", score: str = "", 
                   verified: bool = False, confidence: float = 0, address: str = "", 
                   google_abn: str = "", google_legal_name: str = ""):
        """Add a merchant lookup result with verification data"""
        
        self.rows.append({
            "merchant_name": merchant_name,
            "abn": abn if abn else "",
            "state": state if state else "",
            "legal_name": legal_name if legal_name else "",
            "score": score if score else "",
            "verified": "Yes" if verified else "No",
            "confidence": round(confidence, 2),
            "head_office_address": address if address else "",
            "google_abn": google_abn if google_abn else "",
            "google_legal_name": google_legal_name if google_legal_name else ""
        })

    def save_to_file(self):
        """Save results to file (CSV or XLSX based on extension)"""
        df = pd.DataFrame(self.rows)
        out_path = Path(self.output_file)
        
        if out_path.suffix.lower() == '.xlsx':
            # Save to Excel with formatting
            with pd.ExcelWriter(out_path, engine='openpyxl') as writer:
                df.to_excel(writer, sheet_name='Merchants', index=False)
                
                # Format Excel
                workbook = writer.book
                worksheet = writer.sheets['Merchants']
                
                # Auto-adjust column widths
                for column in worksheet.columns:
                    max_length = 0
                    column_letter = column[0].column_letter
                    for cell in column:
                        try:
                            if len(str(cell.value)) > max_length:
                                max_length = len(str(cell.value))
                        except:
                            pass
                    adjusted_width = min(max_length + 2, 50)
                    worksheet.column_dimensions[column_letter].width = adjusted_width
                
                # Format header row
                from openpyxl.styles import Font, PatternFill
                header_font = Font(bold=True, color="FFFFFF")
                header_fill = PatternFill(start_color="366092", end_color="366092", fill_type="solid")
                
                for cell in worksheet[1]:
                    cell.font = header_font
                    cell.fill = header_fill
        else:
            # Save to CSV
            df.to_csv(out_path, index=False)
        
        return out_path

    def get_dataframe(self) -> pd.DataFrame:
        """Get results as DataFrame"""
        return pd.DataFrame(self.rows)

    def print_summary(self):
        """Print summary statistics including verification data"""
        df = self.get_dataframe()
        total = len(df)
        found = len(df[df["abn"] != ""])
        verified = len(df[df["verified"] == "Yes"])
        with_address = len(df[df["head_office_address"] != ""])
        not_found = total - found
        
        print(f"\n{'='*60}")
        print(f"Complete Verification Pipeline Summary")
        print(f"{'='*60}")
        print(f"Total merchants:            {total}")
        print(f"  ✓ ABN Found:              {found}")
        print(f"  ✗ ABN Not Found:          {not_found}")
        print(f"  • Success rate:           {(found/total*100):.1f}%")
        print(f"\nGoogle Verification:")
        print(f"  ✓ Verified:               {verified}")
        print(f"  • Verification rate:      {(verified/found*100 if found else 0):.1f}%")
        print(f"\nAddress Lookup:")
        print(f"  ✓ Head Office Found:      {with_address}")
        print(f"  • Coverage:               {(with_address/found*100 if found else 0):.1f}%")
        print(f"{'='*60}")
        print(f"{'='*50}\n")
        
        print(df.to_string(index=False))
        print(f"\nOutput saved to: {self.output_file}")


# ================== TROUBLESHOOTING ==================
# Issue: pandas Not Installed
# Solution: pip install pandas
#
# Issue: CSV Permission Denied
# Solution: Close CSV file if open in Excel, check write permissions
#
# Issue: CSV Empty
# Solution: Check if ABR lookups failed in abr_client.py
#
# Issue: Unicode/Encoding Errors
# Solution: File is saved UTF-8, open with encoding='utf-8'
