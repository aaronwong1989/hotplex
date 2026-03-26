#!/usr/bin/env python3
"""
Claude Code stream-json Deep Analysis Script

This script performs multi-turn conversations with Claude Code CLI in stream-json mode
to analyze the complete event structure and identify ALL fields related to context window.

Usage:
    python3 scripts/verify_stream_json_deep.py

Requirements:
    - Claude Code CLI installed and configured
"""

import json
import subprocess
import sys
from typing import Dict, Any, Optional, List
from pathlib import Path


def run_claude_stream_json(prompt: str, session_id: Optional[str] = None, turn: int = 1) -> List[Dict[str, Any]]:
    """
    Run Claude Code with stream-json output format and return parsed events.

    Args:
        prompt: The prompt to send to Claude
        session_id: Optional session ID for multi-turn conversation
        turn: Turn number for logging

    Returns:
        List of parsed JSON events
    """
    cmd = [
        "claude",
        "-p", prompt,
        "--verbose",
        "--output-format", "stream-json"
    ]

    if session_id:
        cmd.extend(["--resume", session_id])

    print(f"\n{'=' * 80}")
    print(f"🔄 Turn #{turn}")
    print(f"{'=' * 80}")
    print(f"🚀 Running: {' '.join(cmd)}")
    print(f"📝 Prompt: {prompt}")
    if session_id:
        print(f"🔑 Session ID: {session_id}")
    print("-" * 80)

    events = []
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=120
        )

        if result.returncode != 0:
            print(f"❌ Command failed with return code {result.returncode}")
            print(f"stderr: {result.stderr}")
            return events

        # Parse each line as a JSON event
        for line_num, line in enumerate(result.stdout.strip().split('\n'), 1):
            if line.strip():
                try:
                    event = json.loads(line)
                    events.append(event)
                except json.JSONDecodeError as e:
                    print(f"⚠️  Line {line_num}: Failed to parse: {e}")
                    print(f"   Content: {line[:100]}...")

    except subprocess.TimeoutExpired:
        print("❌ Command timed out after 120 seconds")
    except Exception as e:
        print(f"❌ Error running command: {e}")

    print(f"✅ Captured {len(events)} events")
    return events


def extract_session_id(events: List[Dict[str, Any]]) -> Optional[str]:
    """Extract session ID from events."""
    for event in events:
        if "session_id" in event:
            return event["session_id"]
    return None


def analyze_all_fields(events: List[Dict[str, Any]], turn: int) -> Dict[str, Any]:
    """
    Analyze ALL fields in ALL events to find context window related data.

    Returns:
        Dictionary containing analysis results
    """
    analysis = {
        "turn": turn,
        "total_events": len(events),
        "event_types": {},
        "context_window_fields": [],
        "usage_fields": [],
        "token_fields": [],
        "full_events_with_usage": [],
    }

    # Track all event types
    for event in events:
        event_type = event.get("type", "unknown")
        analysis["event_types"][event_type] = analysis["event_types"].get(event_type, 0) + 1

    # Search for context window related fields
    context_keywords = [
        "context", "window", "percentage", "used", "remaining", "limit",
        "max_tokens", "context_window"
    ]

    for i, event in enumerate(events):
        event_type = event.get("type", "unknown")

        # Check top-level fields
        for key, value in event.items():
            key_lower = key.lower()
            if any(keyword in key_lower for keyword in context_keywords):
                analysis["context_window_fields"].append({
                    "event_index": i,
                    "event_type": event_type,
                    "field_path": key,
                    "value": value,
                    "value_type": type(value).__name__,
                })

            if "usage" in key_lower or "token" in key_lower:
                analysis["usage_fields"].append({
                    "event_index": i,
                    "event_type": event_type,
                    "field_path": key,
                    "value_type": type(value).__name__,
                })

        # Check nested fields (depth 2)
        for key1, value1 in event.items():
            if isinstance(value1, dict):
                for key2, value2 in value1.items():
                    key_lower = f"{key1}.{key2}".lower()
                    if any(keyword in key_lower for keyword in context_keywords):
                        analysis["context_window_fields"].append({
                            "event_index": i,
                            "event_type": event_type,
                            "field_path": f"{key1}.{key2}",
                            "value": value2,
                            "value_type": type(value2).__name__,
                        })

        # Extract full events with usage data
        if "usage" in event or ("message" in event and "usage" in event.get("message", {})):
            analysis["full_events_with_usage"].append({
                "event_index": i,
                "event_type": event_type,
                "event": event,
            })

    return analysis


def print_detailed_analysis(analysis: Dict[str, Any]) -> None:
    """Print detailed analysis results."""
    print(f"\n{'=' * 80}")
    print(f"📊 Analysis for Turn #{analysis['turn']}")
    print(f"{'=' * 80}")

    print(f"\nEvent Summary:")
    print(f"  • Total events: {analysis['total_events']}")
    print(f"\n  Event type distribution:")
    for event_type, count in sorted(analysis['event_types'].items()):
        print(f"    - {event_type}: {count}")

    print(f"\n{'=' * 80}")
    print(f"🔍 Context Window Related Fields")
    print(f"{'=' * 80}")

    if analysis['context_window_fields']:
        print(f"\nFound {len(analysis['context_window_fields'])} context window related fields:")
        for field in analysis['context_window_fields']:
            print(f"\n  Event #{field['event_index']} ({field['event_type']}):")
            print(f"    • Field: {field['field_path']}")
            print(f"    • Value: {field['value']}")
            print(f"    • Type: {field['value_type']}")
    else:
        print("\n❌ No context window fields found")

    print(f"\n{'=' * 80}")
    print(f"📈 Usage/Token Fields")
    print(f"{'=' * 80}")

    if analysis['usage_fields']:
        print(f"\nFound {len(analysis['usage_fields'])} usage/token related fields:")
        for field in analysis['usage_fields'][:10]:  # Limit output
            print(f"  • Event #{field['event_index']} ({field['event_type']}): {field['field_path']}")
        if len(analysis['usage_fields']) > 10:
            print(f"  ... and {len(analysis['usage_fields']) - 10} more")

    print(f"\n{'=' * 80}")
    print(f"📦 Full Events with Usage Data")
    print(f"{'=' * 80}")

    for event_data in analysis['full_events_with_usage']:
        print(f"\nEvent #{event_data['event_index']} ({event_data['event_type']}):")
        print("-" * 80)
        print(json.dumps(event_data['event'], indent=2, sort_keys=False))
        print("-" * 80)


def calculate_context_percentage(usage_data: Dict[str, int], context_window: int = 200000) -> Optional[float]:
    """
    Calculate context window usage percentage from usage data.

    Formula (based on research):
    total_context = input_tokens + cache_read_input_tokens + cache_creation_input_tokens
    percentage = (total_context / context_window) * 100
    """
    input_tokens = usage_data.get('input_tokens', 0)
    cache_read = usage_data.get('cache_read_input_tokens', 0)
    cache_write = usage_data.get('cache_creation_input_tokens', 0)

    total_context = input_tokens + cache_read + cache_write

    if total_context > 0:
        return (total_context / context_window) * 100
    return None


def main():
    """Main entry point."""
    print("=" * 80)
    print("Claude Code stream-json Deep Analysis")
    print("=" * 80)
    print("\nThis script will perform 3 turns of conversation to analyze context window data")

    all_analyses = []
    session_id = None

    # Turn 1: Simple question
    events1 = run_claude_stream_json(
        "What is 2+2? Just answer with the number.",
        session_id=session_id,
        turn=1
    )
    if events1:
        session_id = extract_session_id(events1)
        analysis1 = analyze_all_fields(events1, turn=1)
        all_analyses.append(analysis1)
        print_detailed_analysis(analysis1)

    # Turn 2: Longer question (to increase context)
    if session_id:
        events2 = run_claude_stream_json(
            "Please explain the concept of recursion in programming, with a simple example.",
            session_id=session_id,
            turn=2
        )
        if events2:
            analysis2 = analyze_all_fields(events2, turn=2)
            all_analyses.append(analysis2)
            print_detailed_analysis(analysis2)

    # Turn 3: Even longer question (to further increase context)
    if session_id:
        events3 = run_claude_stream_json(
            "Now compare recursion with iteration. What are the pros and cons of each? Give examples.",
            session_id=session_id,
            turn=3
        )
        if events3:
            analysis3 = analyze_all_fields(events3, turn=3)
            all_analyses.append(analysis3)
            print_detailed_analysis(analysis3)

    # Final summary
    print(f"\n{'=' * 80}")
    print(f"📋 FINAL SUMMARY")
    print(f"{'=' * 80}")

    print(f"\nContext Window Fields Discovery:")
    total_context_fields = sum(len(a['context_window_fields']) for a in all_analyses)
    print(f"  • Total context window related fields found: {total_context_fields}")

    if total_context_fields > 0:
        print(f"\n  ✅ Context window fields ARE present in stream-json output")
        print(f"\n  Field breakdown by turn:")
        for analysis in all_analyses:
            print(f"    - Turn #{analysis['turn']}: {len(analysis['context_window_fields'])} fields")

        # Show unique field paths
        all_field_paths = set()
        for analysis in all_analyses:
            for field in analysis['context_window_fields']:
                all_field_paths.add(field['field_path'])

        print(f"\n  Unique field paths found:")
        for path in sorted(all_field_paths):
            print(f"    • {path}")
    else:
        print(f"\n  ❌ NO context window fields found in any turn")
        print(f"\n  ⚠️  This means we MUST calculate from token counts")

    # Token-based calculation verification
    print(f"\n{'=' * 80}")
    print(f"🔢 Token-Based Calculation Verification")
    print(f"{'=' * 80}")

    for analysis in all_analyses:
        print(f"\nTurn #{analysis['turn']}:")

        for event_data in analysis['full_events_with_usage']:
            event = event_data['event']

            # Extract usage data
            usage = event.get('usage') or event.get('message', {}).get('usage', {})

            if usage:
                print(f"  Event #{event_data['event_index']} ({event_data['event_type']}):")

                input_tokens = usage.get('input_tokens', 0)
                cache_read = usage.get('cache_read_input_tokens', 0)
                cache_write = usage.get('cache_creation_input_tokens', 0)
                output_tokens = usage.get('output_tokens', 0)

                print(f"    • Input tokens: {input_tokens}")
                print(f"    • Cache read tokens: {cache_read}")
                print(f"    • Cache write tokens: {cache_write}")
                print(f"    • Output tokens: {output_tokens}")

                percentage = calculate_context_percentage(usage)
                if percentage is not None:
                    print(f"    • Calculated context usage: {percentage:.3f}%")

                    # Check if there's a direct percentage field to compare
                    direct_pct = event.get('used_percentage') or event.get('context_used_percentage')
                    if direct_pct is not None:
                        print(f"    • Direct percentage field: {direct_pct}%")
                        diff = abs(percentage - direct_pct)
                        if diff < 0.1:
                            print(f"    ✅ MATCH (diff < 0.1%)")
                        else:
                            print(f"    ⚠️  DIFFERENCE: {diff:.3f}%")

    print(f"\n{'=' * 80}")
    print(f"✅ Analysis Complete")
    print(f"{'=' * 80}")


if __name__ == "__main__":
    main()
