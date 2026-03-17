#!/bin/bash
# diagnose-all.sh - Comprehensive Docker container diagnostics
# Usage: ./diagnose-all.sh [layer] [container]
# Layers: 1=health, 2=resources, 3=logs, all=complete

set -e

LAYER="${1:-all}"
TARGET="${2:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

header() {
    echo -e "\n${CYAN}=== $1 ===${NC}\n"
}

layer1_health() {
    header "Layer 1: Container Health"

    echo -e "${YELLOW}Container States:${NC}"
    docker ps -a --format "{{.Names}}\t{{.Status}}\t{{.Image}}" | column -t

    echo -e "\n${YELLOW}Health Status (containers with healthchecks):${NC}"
    for c in $(docker ps --format "{{.Names}}"); do
        health=$(docker inspect "$c" --format='{{.State.Health.Status}}' 2>/dev/null || echo "n/a")
        case $health in
            healthy) echo -e "  $c: ${GREEN}$health${NC}" ;;
            unhealthy) echo -e "  $c: ${RED}$health${NC}" ;;
            *) echo -e "  $c: $health" ;;
        esac
    done

    echo -e "\n${YELLOW}State Summary:${NC}"
    docker ps -a --format "{{.State}}" | sort | uniq -c | sort -rn | while read count state; do
        echo "  $state: $count"
    done

    echo -e "\n${YELLOW}Restart History (containers with restarts):${NC}"
    for c in $(docker ps -a --format "{{.Names}}"); do
        restarts=$(docker inspect "$c" --format='{{.RestartCount}}' 2>/dev/null || echo "0")
        [ "$restarts" -gt 0 ] && echo -e "  ${RED}$c: $restarts restarts${NC}"
    done
}

layer2_resources() {
    header "Layer 2: Resource Usage"

    echo -e "${YELLOW}CPU & Memory (All Running Containers):${NC}"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}"

    echo -e "\n${YELLOW}Top 5 CPU Consumers:${NC}"
    docker stats --no-stream --format "{{.Name}}\t{{.CPUPerc}}" | sort -t$'\t' -k2 -rn | head -6

    echo -e "\n${YELLOW}Top 5 Memory Consumers:${NC}"
    docker stats --no-stream --format "{{.Name}}\t{{.MemPerc}}" | sort -t$'\t' -k2 -rn | head -6
}

layer3_logs() {
    header "Layer 3: Log Analysis"

    echo -e "${YELLOW}Recent Errors (All Containers, Last 1h):${NC}"
    for c in $(docker ps --format "{{.Names}}"); do
        errors=$(docker logs "$c" --since 1h 2>&1 | grep -ciE "error|fatal|panic|exception" || true)
        if [ "$errors" -gt 0 ]; then
            echo -e "  ${RED}$c: $errors errors${NC}"
        fi
    done

    echo -e "\n${YELLOW}Recent Warnings (All Containers, Last 1h):${NC}"
    for c in $(docker ps --format "{{.Names}}"); do
        warnings=$(docker logs "$c" --since 1h 2>&1 | grep -ciE "warn|warning" || true)
        if [ "$warnings" -gt 0 ]; then
            echo -e "  ${YELLOW}$c: $warnings warnings${NC}"
        fi
    done
}

hotplex_deep_dive() {
    header "HotPlex Deep Dive"

    COMPOSE_DIR="$HOME/hotplex/docker/matrix"

    if [ -d "$COMPOSE_DIR" ]; then
        echo -e "${YELLOW}HotPlex Container Status:${NC}"
        cd "$COMPOSE_DIR" && docker compose ps 2>/dev/null || echo "  No HotPlex containers found"

        echo -e "\n${YELLOW}HTTP Health Endpoints:${NC}"
        for port in 18080 18081 18082; do
            status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$port/health 2>/dev/null || echo "000")
            case $status in
                200) echo -e "  Port $port: ${GREEN}HTTP $status${NC}" ;;
                000) echo -e "  Port $port: ${RED}Connection refused${NC}" ;;
                *) echo -e "  Port $port: ${YELLOW}HTTP $status${NC}" ;;
            esac
        done

        echo -e "\n${YELLOW}Socket Mode Status:${NC}"
        cd "$COMPOSE_DIR" && docker compose logs --tail=50 2>&1 | grep -E "Socket Mode|Connected|Disconnected" | tail -5
    else
        echo "  HotPlex not found at $COMPOSE_DIR"
    fi
}

single_container() {
    local container="$1"
    header "Deep Dive: $container"

    if ! docker ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
        echo -e "${RED}Container '$container' not found${NC}"
        return 1
    fi

    echo -e "${YELLOW}Status:${NC}"
    docker inspect "$container" --format='Status: {{.State.Status}}, Health: {{.State.Health.Status}}, Restarts: {{.RestartCount}}, Started: {{.State.StartedAt}}' 2>/dev/null

    echo -e "\n${YELLOW}Resources:${NC}"
    docker stats --no-stream "$container" --format "CPU: {{.CPUPerc}}, Memory: {{.MemUsage}}, Network: {{.NetIO}}" 2>/dev/null

    echo -e "\n${YELLOW}Mounts:${NC}"
    docker inspect "$container" --format='{{range .Mounts}}  {{.Source}} -> {{.Destination}}{{println}}{{end}}' 2>/dev/null | head -5

    echo -e "\n${YELLOW}Recent Logs (Last 30 lines):${NC}"
    docker logs "$container" --tail=30 2>&1
}

# Main execution
case "$LAYER" in
    1) layer1_health ;;
    2) layer2_resources ;;
    3) layer3_logs ;;
    all)
        layer1_health
        layer2_resources
        layer3_logs
        hotplex_deep_dive
        ;;
    *)
        # Assume it's a container name
        single_container "$LAYER"
        ;;
esac

if [ -n "$TARGET" ]; then
    single_container "$TARGET"
fi
