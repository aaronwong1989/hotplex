#!/usr/bin/env python3
"""
Claude Code modelUsage Complete Field Analysis

This script performs detailed analysis of ALL fields in modelUsage structure
to understand the complete information available for context window calculation.

Usage:
    python3 scripts/analyze_model_usage_fields.py
"""

import json
import subprocess
import sys
from typing import Dict, Any, Optional, List


def run_claude_and_collect(prompt: str) -> Dict[str, Any]:
    """Run Claude Code and collect complete result event."""
    cmd = [
        "claude",
        "-p", prompt,
        "--verbose",
        "--output-format", "stream-json"
    ]

    print(f"🚀 Running: {' '.join(cmd)}")
    print(f"📝 Prompt: {prompt}")
    print("-" * 80)

    result_event = None
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=120
        )

        if result.returncode != 0:
            print(f"❌ Command failed: {result.stderr}")
            return {}

        # Find the result event
        for line in result.stdout.strip().split('\n'):
            if line.strip():
                try:
                    event = json.loads(line)
                    if event.get("type") == "result":
                        result_event = event
                        break
                except json.JSONDecodeError as e:
                    print(f"⚠️  Parse error: {e}")

    except subprocess.TimeoutExpired:
        print("❌ Timeout")
    except Exception as e:
        print(f"❌ Error: {e}")

    return result_event or {}


def analyze_usage_fields(event: Dict[str, Any]) -> None:
    """Analyze ALL usage-related fields in the event."""
    print("\n" + "=" * 80)
    print("📊 COMPLETE FIELD ANALYSIS")
    print("=" * 80)

    # 1. Top-level usage
    print("\n🔹 TOP-LEVEL usage FIELD:")
    print("-" * 80)
    if "usage" in event:
        usage = event["usage"]
        print(json.dumps(usage, indent=2, sort_keys=False))
    else:
        print("  ❌ Not present")

    # 2. modelUsage - the key field
    print("\n🔹 TOP-LEVEL modelUsage FIELD:")
    print("-" * 80)
    if "modelUsage" in event:
        model_usage = event["modelUsage"]
        for model_name, stats in model_usage.items():
            print(f"\n  Model: {model_name}")
            print("  " + "-" * 76)
            for key, value in sorted(stats.items()):
                value_type = type(value).__name__
                print(f"    • {key:30s} = {value:20} ({value_type})")
    else:
        print("  ❌ Not present")

    # 3. Check for ANY other token-related fields
    print("\n🔹 ALL TOKEN-RELATED FIELDS (recursive search):")
    print("-" * 80)

    def find_token_fields(obj: Any, path: str = "", depth: int = 0):
        if depth > 3:  # Limit recursion depth
            return

        if isinstance(obj, dict):
            for key, value in obj.items():
                current_path = f"{path}.{key}" if path else key
                key_lower = key.lower()

                # Check for token/context/cost related fields
                if any(keyword in key_lower for keyword in
                       ["token", "context", "window", "cost", "usage", "cache"]):
                    value_type = type(value).__name__
                    print(f"  • {current_path:50s} = {value!r:30} ({value_type})")

                # Recurse into nested objects
                if isinstance(value, dict):
                    find_token_fields(value, current_path, depth + 1)

    find_token_fields(event)

    # 4. Show complete event structure (all top-level keys)
    print("\n🔹 ALL TOP-LEVEL EVENT FIELDS:")
    print("-" * 80)
    for key in sorted(event.keys()):
        value = event[key]
        value_type = type(value).__name__
        if isinstance(value, (dict, list)):
            size = len(value)
            print(f"  • {key:30s} = ({value_type}, {size} items)")
        else:
            print(f"  • {key:30s} = {value!r:30} ({value_type})")


def verify_calculation(event: Dict[str, Any]) -> None:
    """Verify context window calculation with available data."""
    print("\n" + "=" * 80)
    print("🔢 CONTEXT WINDOW CALCULATION VERIFICATION")
    print("=" * 80)

    if "modelUsage" not in event:
        print("❌ No modelUsage field found")
        return

    for model_name, stats in event["modelUsage"].items():
        print(f"\n📌 Model: {model_name}")
        print("-" * 80)

        # Extract fields
        input_tokens = stats.get("inputTokens", 0)
        output_tokens = stats.get("outputTokens", 0)
        cache_read = stats.get("cacheReadInputTokens", 0)
        cache_write = stats.get("cacheCreationInputTokens", 0)
        context_window = stats.get("contextWindow", 0)
        max_output = stats.get("maxOutputTokens", 0)
        cost_usd = stats.get("costUSD", 0.0)

        print(f"\n  Available fields:")
        print(f"    • inputTokens:              {input_tokens:>10,}")
        print(f"    • outputTokens:             {output_tokens:>10,}")
        print(f"    • cacheReadInputTokens:     {cache_read:>10,}")
        print(f"    • cacheCreationInputTokens: {cache_write:>10,}")
        print(f"    • contextWindow:            {context_window:>10,}")
        print(f"    • maxOutputTokens:          {max_output:>10,}")
        print(f"    • costUSD:                  {cost_usd:>10.6f}")

        # Calculate context percentage
        if context_window > 0:
            total_input = input_tokens + cache_read + cache_write
            percentage = (total_input / context_window) * 100

            print(f"\n  Calculation:")
            print(f"    • Total input tokens: {total_input:,}")
            print(f"      = {input_tokens:,} (input) + {cache_read:,} (cache_read) + {cache_write:,} (cache_write)")
            print(f"    • Context window size: {context_window:,}")
            print(f"    • Used percentage: {percentage:.4f}%")

            # Context remaining
            remaining = context_window - total_input
            remaining_pct = 100 - percentage
            print(f"    • Remaining tokens: {remaining:,} ({remaining_pct:.4f}%)")
        else:
            print(f"\n  ⚠️  contextWindow = 0 (cannot calculate percentage)")


def main():
    """Main entry point."""
    print("=" * 80)
    print("Claude Code modelUsage Complete Field Analysis")
    print("=" * 80)

    # Test with a moderate-length prompt to get meaningful data
    prompt = """Please analyze the following code and provide a detailed review:

```go
func factorial(n int) int {
    if n <= 1 {
        return 1
    }
    return n * factorial(n-1)
}
```

Consider: correctness, performance, edge cases, and potential improvements."""

    event = run_claude_and_collect(prompt)

    if not event:
        print("\n❌ No result event captured")
        sys.exit(1)

    analyze_usage_fields(event)
    verify_calculation(event)

    print("\n" + "=" * 80)
    print("✅ Analysis Complete")
    print("=" * 80)


if __name__ == "__main__":
    main()
