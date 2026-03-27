#!/usr/bin/env python3
"""
OpenCode Server Provider Verification Script

This script verifies that the OpenCode Server provider is working correctly
by testing all HTTP/SSE endpoints.

Usage:
    python3 scripts/test_opencode_server.py [--url URL] [--password PASSWORD]

Examples:
    # Test default localhost server
    python3 scripts/test_opencode_server.py

    # Test remote server
    python3 scripts/test_opencode_server.py --url http://192.168.1.100:4096

    # Test with Basic Auth
    python3 scripts/test_opencode_server.py --password $(cat ~/.hotplex/.opencode-password)
"""

import argparse
import json
import sys
import time
from typing import Optional

try:
    import requests
    from requests.auth import HTTPBasicAuth
except ImportError:
    print("❌ Error: requests library not installed")
    print("   Install: pip install requests")
    sys.exit(1)


class Colors:
    """ANSI color codes for terminal output"""
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    RESET = '\033[0m'
    BOLD = '\033[1m'


class OpenCodeServerTester:
    """Test OpenCode Server HTTP/SSE endpoints"""

    def __init__(self, base_url: str, password: Optional[str] = None):
        self.base_url = base_url.rstrip('/')
        self.auth = HTTPBasicAuth('', password) if password else None
        self.session = requests.Session()
        self.session_id: Optional[str] = None

    def print_header(self, title: str):
        """Print a formatted test header"""
        print(f"\n{Colors.CYAN}{Colors.BOLD}{'═' * 60}{Colors.RESET}")
        print(f"{Colors.CYAN}{Colors.BOLD}  {title}{Colors.RESET}")
        print(f"{Colors.CYAN}{Colors.BOLD}{'═' * 60}{Colors.RESET}\n")

    def print_result(self, test_name: str, success: bool, details: str = ""):
        """Print test result with color coding"""
        status = f"{Colors.GREEN}✅ PASS{Colors.RESET}" if success else f"{Colors.RED}❌ FAIL{Colors.RESET}"
        print(f"{status} {test_name}")
        if details:
            print(f"     {Colors.BLUE}{details}{Colors.RESET}")

    def test_health_check(self) -> bool:
        """Test 1: Health check endpoint"""
        self.print_header("Test 1: Health Check")

        try:
            print(f"→ GET {self.base_url}/")
            start = time.time()
            response = self.session.get(
                f"{self.base_url}/",
                auth=self.auth,
                timeout=5
            )
            elapsed = (time.time() - start) * 1000

            if response.status_code == 200:
                self.print_result("Health check", True, f"HTTP {response.status_code} ({elapsed:.0f}ms)")
                return True
            else:
                self.print_result("Health check", False, f"HTTP {response.status_code}")
                return False

        except requests.exceptions.Timeout:
            self.print_result("Health check", False, "Timeout after 5s")
            return False
        except requests.exceptions.ConnectionError as e:
            self.print_result("Health check", False, f"Connection refused: {e}")
            return False
        except Exception as e:
            self.print_result("Health check", False, f"Error: {e}")
            return False

    def test_create_session(self) -> bool:
        """Test 2: Create session"""
        self.print_header("Test 2: Create Session")

        try:
            payload = {
                "provider": "anthropic",
                "model": "claude-sonnet-4-20250514"
            }

            print(f"→ POST {self.base_url}/session")
            print(f"  Payload: {json.dumps(payload, indent=2)}")

            start = time.time()
            response = self.session.post(
                f"{self.base_url}/session",
                json=payload,
                auth=self.auth,
                timeout=10
            )
            elapsed = (time.time() - start) * 1000

            if response.status_code == 200 or response.status_code == 201:
                data = response.json()
                self.session_id = data.get("id") or data.get("sessionId") or data.get("session_id")

                self.print_result(
                    "Create session",
                    True,
                    f"Session ID: {self.session_id} ({elapsed:.0f}ms)"
                )
                return True
            else:
                self.print_result(
                    "Create session",
                    False,
                    f"HTTP {response.status_code}: {response.text[:200]}"
                )
                return False

        except Exception as e:
            self.print_result("Create session", False, f"Error: {e}")
            return False

    def test_send_message(self) -> bool:
        """Test 3: Send message"""
        self.print_header("Test 3: Send Message")

        if not self.session_id:
            self.print_result("Send message", False, "No session ID (create session first)")
            return False

        try:
            payload = {
                "parts": [{
                    "type": "text",
                    "text": "Hello! Please respond with 'OK' to confirm you're working."
                }]
            }

            print(f"→ POST {self.base_url}/session/{self.session_id}/message")
            print(f"  Payload: {json.dumps(payload, indent=2)}")

            start = time.time()
            response = self.session.post(
                f"{self.base_url}/session/{self.session_id}/message",
                json=payload,
                auth=self.auth,
                timeout=30
            )
            elapsed = (time.time() - start) * 1000

            if response.status_code == 200 or response.status_code == 201:
                self.print_result("Send message", True, f"Message sent ({elapsed:.0f}ms)")

                # Try to parse response
                try:
                    data = response.json()
                    print(f"\n{Colors.BLUE}  Response:{Colors.RESET}")
                    print(f"  {json.dumps(data, indent=2)[:500]}")
                except:
                    print(f"\n{Colors.BLUE}  Response (text):{Colors.RESET}")
                    print(f"  {response.text[:200]}")

                return True
            else:
                self.print_result(
                    "Send message",
                    False,
                    f"HTTP {response.status_code}: {response.text[:200]}"
                )
                return False

        except Exception as e:
            self.print_result("Send message", False, f"Error: {e}")
            return False

    def test_sse_stream(self) -> bool:
        """Test 4: SSE event stream"""
        self.print_header("Test 4: SSE Event Stream")

        try:
            print(f"→ GET {self.base_url}/event (SSE stream)")
            print(f"  Listening for events (up to 10 seconds)...\n")

            start = time.time()
            event_count = 0
            max_events = 5  # Stop after receiving 5 events
            timeout_seconds = 10

            response = self.session.get(
                f"{self.base_url}/event",
                auth=self.auth,
                stream=True,
                timeout=15
            )

            if response.status_code != 200:
                self.print_result("SSE stream", False, f"HTTP {response.status_code}")
                return False

            # Read SSE events
            for line in response.iter_lines(decode_unicode=True):
                elapsed = time.time() - start

                # Stop after timeout
                if elapsed > timeout_seconds:
                    break

                if line:
                    event_count += 1
                    # Color-code event data
                    if line.startswith('data:'):
                        print(f"  {Colors.CYAN}[{event_count}]{Colors.RESET} {line[:100]}")
                    else:
                        print(f"  {Colors.BLUE}[{event_count}]{Colors.RESET} {line[:100]}")

                    # Stop after receiving enough events
                    if event_count >= max_events:
                        break

            # Consider test passed if we received at least 1 event
            if event_count >= 1:
                self.print_result("SSE stream", True, f"Received {event_count} events in {elapsed:.1f}s")
                return True
            else:
                self.print_result("SSE stream", False, f"No events in {timeout_seconds}s")
                return False

        except requests.exceptions.Timeout:
            # Timeout is acceptable if we got some events
            elapsed = time.time() - start
            if event_count >= 1:
                self.print_result("SSE stream", True, f"Received {event_count} events before timeout")
                return True
            else:
                self.print_result("SSE stream", False, f"Timeout after {timeout_seconds}s (no events)")
                return False
        except Exception as e:
            self.print_result("SSE stream", False, f"Error: {e}")
            return False

    def test_delete_session(self) -> bool:
        """Test 5: Delete session"""
        self.print_header("Test 5: Delete Session")

        if not self.session_id:
            self.print_result("Delete session", False, "No session ID (create session first)")
            return False

        try:
            print(f"→ DELETE {self.base_url}/session/{self.session_id}")

            start = time.time()
            response = self.session.delete(
                f"{self.base_url}/session/{self.session_id}",
                auth=self.auth,
                timeout=10
            )
            elapsed = (time.time() - start) * 1000

            # 200, 204, or 404 are acceptable
            if response.status_code in [200, 204, 404]:
                status_note = " (session already deleted)" if response.status_code == 404 else ""
                self.print_result(
                    "Delete session",
                    True,
                    f"HTTP {response.status_code}{status_note} ({elapsed:.0f}ms)"
                )
                return True
            else:
                self.print_result(
                    "Delete session",
                    False,
                    f"HTTP {response.status_code}: {response.text[:200]}"
                )
                return False

        except Exception as e:
            self.print_result("Delete session", False, f"Error: {e}")
            return False

    def run_all_tests(self) -> dict:
        """Run all tests and return results"""
        print(f"\n{Colors.BOLD}{Colors.BLUE}╭─ 🔍 OpenCode Server Verification ─────────────{Colors.RESET}")
        print(f"{Colors.BOLD}{Colors.BLUE}│{Colors.RESET}")
        print(f"{Colors.BOLD}{Colors.BLUE}│  Server: {self.base_url}{Colors.RESET}")
        print(f"{Colors.BOLD}{Colors.BLUE}│  Auth: {'Enabled' if self.auth else 'Disabled'}{Colors.RESET}")
        print(f"{Colors.BOLD}{Colors.BLUE}╰───────────────────────────────────────────────{Colors.RESET}")

        results = {
            "health_check": self.test_health_check(),
            "create_session": self.test_create_session(),
            "send_message": self.test_send_message(),
            "sse_stream": self.test_sse_stream(),
            "delete_session": self.test_delete_session(),
        }

        # Summary
        self.print_header("Test Summary")
        passed = sum(results.values())
        total = len(results)

        for test_name, success in results.items():
            self.print_result(test_name.replace("_", " ").title(), success)

        print(f"\n{Colors.BOLD}{'═' * 60}{Colors.RESET}")
        if passed == total:
            print(f"{Colors.GREEN}{Colors.BOLD}✅ All tests passed ({passed}/{total}){Colors.RESET}")
            print(f"{Colors.GREEN}{Colors.BOLD}   OpenCode Server is working correctly!{Colors.RESET}")
        else:
            print(f"{Colors.RED}{Colors.BOLD}❌ Some tests failed ({passed}/{total} passed){Colors.RESET}")
            print(f"{Colors.RED}{Colors.BOLD}   Check OpenCode Server logs for details{Colors.RESET}")
        print(f"{Colors.BOLD}{'═' * 60}{Colors.RESET}\n")

        return results


def main():
    parser = argparse.ArgumentParser(
        description="Verify OpenCode Server provider functionality",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Test default localhost server
  python3 scripts/test_opencode_server.py

  # Test remote server
  python3 scripts/test_opencode_server.py --url http://192.168.1.100:4096

  # Test with Basic Auth
  python3 scripts/test_opencode_server.py --password $(cat ~/.hotplex/.opencode-password)
        """
    )
    parser.add_argument(
        "--url",
        default="http://127.0.0.1:4096",
        help="OpenCode Server URL (default: http://127.0.0.1:4096)"
    )
    parser.add_argument(
        "--password",
        help="Password for Basic Auth (optional)"
    )

    args = parser.parse_args()

    tester = OpenCodeServerTester(args.url, args.password)
    results = tester.run_all_tests()

    # Exit with error code if any test failed
    sys.exit(0 if all(results.values()) else 1)


if __name__ == "__main__":
    main()
