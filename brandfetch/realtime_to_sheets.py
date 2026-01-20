"""
Realtime listener: streams Supabase table changes into a Google Sheet.

Env vars required:
- SUPABASE_URL: https://<project>.supabase.co
- SUPABASE_KEY: anon or service role (service role only on server/back-end)
- SUPABASE_SCHEMA: default "public"
- SUPABASE_TABLE: default "enriched_merchants"
- GOOGLE_SHEETS_ID: target spreadsheet ID
- GOOGLE_SHEETS_RANGE: e.g. "Sheet1!A:Z" (default)
- GOOGLE_SERVICE_ACCOUNT_FILE: path to service account JSON with Sheets access

Prereqs in Supabase:
- Table must be added to publication: alter publication supabase_realtime add table public.enriched_merchants;
- Policies must allow the role used by SUPABASE_KEY to read changes.
"""

import asyncio
import json
import os
import signal
import sys
import time
from typing import Any, Dict, List

import websockets
from google.oauth2.service_account import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError


SUPABASE_URL = os.getenv("SUPABASE_URL", "").rstrip("/")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")
SUPABASE_SCHEMA = os.getenv("SUPABASE_SCHEMA", "public")
SUPABASE_TABLE = os.getenv("SUPABASE_TABLE", "enriched_merchants")
GOOGLE_SHEETS_ID = os.getenv("GOOGLE_SHEETS_ID")
GOOGLE_SHEETS_RANGE = os.getenv("GOOGLE_SHEETS_RANGE", "Sheet1!A:Z")
GOOGLE_SERVICE_ACCOUNT_FILE = os.getenv("GOOGLE_SERVICE_ACCOUNT_FILE", "service-account.json")


def require(value: str, name: str) -> str:
    if not value:
        print(f"Missing required env: {name}", file=sys.stderr)
        sys.exit(1)
    return value


def sheets_client():
    creds = Credentials.from_service_account_file(
        GOOGLE_SERVICE_ACCOUNT_FILE,
        scopes=["https://www.googleapis.com/auth/spreadsheets"],
    )
    return build("sheets", "v4", credentials=creds).spreadsheets().values()


def make_row(payload: Dict[str, Any]) -> List[Any]:
    # Use the "record" for INSERT/UPDATE, "old_record" for DELETE
    record = payload.get("record") or payload.get("old_record") or {}
    return [
        record.get("transaction_cache", ""),
        record.get("brand_name", ""),
        record.get("website_url", ""),
        record.get("confidence_score", ""),
        record.get("brandfetch_id", ""),
        json.dumps(record),
        time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    ]


async def append_row(values_api, row: List[Any]):
    try:
        await asyncio.to_thread(
            values_api.append,
            spreadsheetId=GOOGLE_SHEETS_ID,
            range=GOOGLE_SHEETS_RANGE,
            valueInputOption="USER_ENTERED",
            body={"values": [row]},
        )
        print("Appended:", row)
    except HttpError as err:
        print("Sheets append error:", err, file=sys.stderr)


async def heartbeat(ws, interval: int = 15):
    ref = 0
    while True:
        await asyncio.sleep(interval)
        ref += 1
        await ws.send(json.dumps({"topic": "phoenix", "event": "heartbeat", "payload": {}, "ref": ref}))


async def listen_and_forward():
    require(SUPABASE_URL, "SUPABASE_URL")
    require(SUPABASE_KEY, "SUPABASE_KEY")
    require(GOOGLE_SHEETS_ID, "GOOGLE_SHEETS_ID")
    require(GOOGLE_SERVICE_ACCOUNT_FILE, "GOOGLE_SERVICE_ACCOUNT_FILE")

    ws_url = f"{SUPABASE_URL.replace('https', 'wss')}/realtime/v1/websocket?apikey={SUPABASE_KEY}&vsn=1.0.0"
    topic = f"realtime:{SUPABASE_SCHEMA}:{SUPABASE_TABLE}"
    join_payload = {
        "topic": topic,
        "event": "phx_join",
        "payload": {
            "events": ["*"],
            "postgres_changes": [
                {"event": "*", "schema": SUPABASE_SCHEMA, "table": SUPABASE_TABLE}
            ],
        },
        "ref": 1,
    }

    values_api = sheets_client()

    async for ws in websockets.connect(ws_url, ping_interval=None):
        try:
            await ws.send(json.dumps(join_payload))
            hb_task = asyncio.create_task(heartbeat(ws))
            async for message in ws:
                data = json.loads(message)
                if data.get("event") == "postgres_changes":
                    row = make_row(data.get("payload", {}))
                    await append_row(values_api, row)
        except websockets.ConnectionClosed:
            print("Realtime connection closed. Reconnecting...", file=sys.stderr)
            await asyncio.sleep(2)
        except Exception as exc:  # pylint: disable=broad-except
            print("Listener error:", exc, file=sys.stderr)
            await asyncio.sleep(5)
        finally:
            if not ws.closed:
                await ws.close()
            if 'hb_task' in locals():
                hb_task.cancel()


def main():
    loop = asyncio.get_event_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, loop.stop)
    loop.create_task(listen_and_forward())
    loop.run_forever()


if __name__ == "__main__":
    main()
