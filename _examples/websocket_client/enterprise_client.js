/**
 * enterprise_client.js
 *
 * HotPlex Enterprise Gateway Reference Client
 * Demonstrates full-duplex communication, async task management, and telemetry.
 *
 * Capabilities demonstrated:
 * 1. Version Handshake
 * 2. System Prompt Injection
 * 3. Streaming Event Processing (Thinking, Tool, Answer)
 * 4. Manual Interruption (Stop Command)
 * 5. Telemetry & Stats Retrieval
 */

const WebSocket = require("ws");

// Configuration
const GATEWAY_URL = "ws://localhost:8080/ws/v1/agent";
const SESSION_ID = "enterprise-demo-" + Math.floor(Math.random() * 1000);

const ws = new WebSocket(GATEWAY_URL);

// --- State Tracking ---
let currentStep = "version";

console.log(
  `\x1b[36m[HotPlex] Initializing Connection to ${GATEWAY_URL}...\x1b[0m`,
);

ws.on("open", () => {
  console.log(
    `\x1b[32m[✓] Connection established. Session: ${SESSION_ID}\x1b[0m\n`,
  );

  // Step 1: Query Engine Version
  sendRequest({ type: "version" });
});

ws.on("message", (rawData) => {
  const message = JSON.parse(rawData);
  const { event, data } = message;

  switch (event) {
    case "version":
      handleVersion(data);
      break;
    case "thinking":
      console.log(
        `\x1b[33m[AI] 🤔 Thinking: ${data.EventData || "Planning next steps..."}\x1b[0m`,
      );
      break;
    case "tool_use":
      console.log(`\x1b[34m[Tool] 🛠️  Executing: ${data.EventData}\x1b[0m`);
      break;
    case "answer":
      process.stdout.write(`\x1b[37m${data.EventData}\x1b[0m`);
      break;
    case "completed":
      console.log(`\n\n\x1b[32m[✓] Task Completed.\x1b[0m`);
      // Step 3: Request Detailed Telemetry
      sendRequest({ type: "stats", session_id: SESSION_ID });
      break;
    case "stats":
      handleStats(data);
      break;
    case "stopped":
      console.log(
        `\n\x1b[31m[!] Execution Interrupted: ${data.session_id}\x1b[0m`,
      );
      process.exit(0);
    case "error":
      console.error(`\n\x1b[41m[Error]\x1b[0m ${data.message}`);
      ws.close();
      break;
  }
});

// --- Logic Handlers ---

function handleVersion(data) {
  console.log(`\x1b[35m[System] Engine Version: ${data.version.trim()}\x1b[0m`);

  // Step 2: Start a Multi-step Execution
  runComplexTask();
}

function runComplexTask() {
  console.log(
    `\x1b[36m[Client] Sending task with System Prompt injection...\x1b[0m\n`,
  );

  const payload = {
    type: "execute",
    session_id: SESSION_ID,
    prompt:
      'Use the "ls" command to list files in the current directory, then summarize what you see.',
    system_prompt:
      "You are a Senior DevOps Auditor. Be concise and professional.",
    work_dir: process.cwd(),
  };

  sendRequest(payload);

  // Optional: Demonstrate "Stop" capability after 5 seconds if still running
  // setTimeout(() => {
  //     console.log(`\n\x1b[31m[Client] Timeout reached. Sending STOP command...\x1b[0m`);
  //     sendRequest({ type: 'stop', session_id: SESSION_ID, reason: 'interaction:timeout' });
  // }, 5000);
}

function handleStats(data) {
  console.log(`\n\x1b[36m--- Telemetry Report ---\x1b[0m`);
  console.log(`Tokens (In/Out): ${data.input_tokens} / ${data.output_tokens}`);
  console.log(`Execution Time:  ${data.total_duration_ms}ms`);
  console.log(`Tool Calls:      ${data.tool_call_count}`);
  console.log(`\x1b[36m------------------------\x1b[0m\n`);

  console.log(`\x1b[32m[Final] Ready for next session.\x1b[0m`);
  ws.close();
}

// --- Utils ---

function sendRequest(obj) {
  ws.send(JSON.stringify(obj));
}

ws.on("close", () => {
  console.log(`\n\x1b[90m[System] Connection closed.\x1b[0m`);
});

ws.on("error", (err) => {
  console.error(`\x1b[31m[Socket Error]\x1b[0m ${err.message}`);
});
