// client.js
// A simple Node.js WebSocket client to demonstrate interacting with HotPlex (API v1).
// This serves as a reference for how frontends or other microservices can connect to the HotPlex gateway.
// Important prerequisite: you need to install the 'ws' library first: `npm install ws`

const WebSocket = require('ws');

// 1. Establish the WebSocket Connection
// Connect to the HotPlex WebSocket endpoint. By default, the server runs on port 8080.
const ws = new WebSocket('ws://localhost:8080/ws/v1/agent');

// 2. Handle Connection Open Event
// This event is triggered once the WebSocket handshake is successful.
ws.on('open', function open() {
    console.log('[CLIENT] Connected to HotPlex WebSocket Server');

    // 3. Construct the Execution Payload
    // The payload instructs HotPlex on what to execute and under what context.
    const payload = {
        type: 'execute',                // The action type (currently supports 'execute' and 'stop')
        session_id: 'test-session-123', // Hot-multiplexing session ID. Determines which persistent process to reuse.
        prompt: 'Calculate 25 * 4, print just the result, do not explain.', // The user's input/command
        work_dir: '/tmp'                // The isolated filesystem directory for this task
    };

    console.log('[CLIENT] Sending request:', payload);

    // Send the payload as a JSON string
    ws.send(JSON.stringify(payload));
});

// 4. Handle Incoming Messages (Server -> Client)
// HotPlex streams events back asynchronously as the underlying agent executes.
ws.on('message', function incoming(data) {
    try {
        // Parse the incoming JSON event. Standard payload format is { event: string, data: any }
        const message = JSON.parse(data);
        console.log(`\n[SERVER EVENT: ${message.event}]`);

        // Route the event based on its type
        if (message.event === 'thinking') {
            // The agent is planning or waiting for a model response.
            console.log('🤔 AI is thinking...');
        } else if (message.event === 'answer') {
            // Emitted for direct text responses or fallback raw outputs.
            console.log(message.data);
        } else if (message.event === 'completed') {
            // The execution has successfully finished. Safe to close or send next prompt.
            console.log('✅ Execution completed successfully!');
            ws.close();
        } else if (message.event === 'error') {
            // An error occurred during execution (e.g., config invalid, WAF blocked).
            console.log('❌ Error from server:', message.data);
            ws.close();
        } else if (message.event === 'assistant') {
            // Claude Code specific stream-json event. Contains chunks of the assistant's reply.
            if (message.data && message.data.message && message.data.message.content) {
                const text = message.data.message.content.find(c => c.type === 'text');
                if (text) {
                    // Use process.stdout.write to print inline (without automatic newlines)
                    process.stdout.write(text.text);
                }
            }
        }
    } catch (e) {
        // Fallback for non-JSON messages (rare, but good practice to handle)
        console.log('[SERVER RAW]', data.toString());
    }
});

ws.on('close', function close() {
    console.log('\n[CLIENT] Connection closed');
});

ws.on('error', function error(err) {
    console.error('[CLIENT ERROR]', err);
});
