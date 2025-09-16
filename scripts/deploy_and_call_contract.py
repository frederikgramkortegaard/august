#!/usr/bin/env python3
"""
Simple script to reproduce the gas calculation issue:
1. Start a seed node
2. Mine a block
3. Deploy a contract
4. Mine a block
5. Call the contract
6. Mine a block (this should fail with gas mismatch)
"""

import subprocess
import time
import json
import os
import signal
import threading
import sys

# ANSI color codes
class Colors:
    GREEN = '\033[92m'      # Node output
    RED = '\033[91m'        # Wallet output
    BLUE = '\033[94m'       # Miner output
    YELLOW = '\033[93m'     # Keygen output
    PURPLE = '\033[95m'     # Error output
    CYAN = '\033[96m'       # General commands
    WHITE = '\033[97m'      # Default
    BOLD = '\033[1m'
    END = '\033[0m'         # Reset

def colored_print(text, color):
    """Print text in specified color"""
    print(f"{color}{text}{Colors.END}")

def run_cmd(cmd, capture_output=True, timeout=30, output_color=Colors.WHITE):
    """Run a command and return the result"""
    colored_print(f"Running: {cmd}", Colors.CYAN)
    try:
        if capture_output:
            result = subprocess.run(cmd, shell=True, capture_output=capture_output, text=True, timeout=timeout)

            # Print stdout in the specified color
            if result.stdout:
                colored_print("STDOUT:", output_color)
                colored_print(result.stdout, output_color)

            # Print stderr in purple (error color)
            if result.stderr:
                colored_print("STDERR:", Colors.PURPLE)
                colored_print(result.stderr, Colors.PURPLE)
        else:
            # Run without capturing output - let it print directly to terminal
            result = subprocess.run(cmd, shell=True, timeout=timeout)

        if result.returncode != 0:
            colored_print(f"Command failed with code {result.returncode}", Colors.PURPLE)

        return result
    except subprocess.TimeoutExpired:
        colored_print("Command timed out!", Colors.PURPLE)
        return None

def node_output_reader(process, stop_event):
    """Read node output in a separate thread"""
    try:
        while not stop_event.is_set() and process.poll() is None:
            line = process.stdout.readline()
            if line:
                colored_print(f"NODE: {line.strip()}", Colors.GREEN)
            else:
                time.sleep(0.1)
    except Exception as e:
        colored_print(f"Error reading node output: {e}", Colors.PURPLE)

def main():
    print("=== Debugging Gas Calculation Issue ===")
    os.chdir("/Users/fgk/Developer/august")

    # Build the project
    colored_print("\n1. Building project...", Colors.BOLD)
    build_result = run_cmd("go build ./...", output_color=Colors.WHITE)
    if build_result.returncode != 0:
        colored_print("Build failed!", Colors.PURPLE)
        return

    # Start seed node in background
    colored_print("\n2. Starting seed node...", Colors.BOLD)
    seed_process = subprocess.Popen(
        ["go", "run", "cmd/seed/main.go", "-port", "8333", "-minerport", "8334"],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,  # Merge stderr into stdout
        universal_newlines=True,
        bufsize=1
    )

    # Start thread to read node output
    stop_event = threading.Event()
    output_thread = threading.Thread(target=node_output_reader, args=(seed_process, stop_event))
    output_thread.daemon = True
    output_thread.start()

    # Wait for node to start
    colored_print("Waiting for node to start...", Colors.GREEN)
    time.sleep(2)  # Give node time to start

    try:
        # Generate a private key for the miner
        colored_print("\n3. Generating private key...", Colors.BOLD)
        keygen_result = run_cmd("go run cmd/keygen/main.go", output_color=Colors.YELLOW)
        if keygen_result.returncode != 0:
            colored_print("Failed to generate key!", Colors.PURPLE)
            return

        # Extract private key from output (format: "Private key (64 bytes): <hex>")
        privkey = None
        for line in keygen_result.stdout.split('\n'):
            if line.startswith("Private key"):
                privkey = line.split(": ")[1].strip()
                break

        if not privkey:
            colored_print("Could not extract private key!", Colors.PURPLE)
            return

        colored_print(f"Generated private key: {privkey[:16]}...", Colors.YELLOW)

        # Mine first block to get some coins
        colored_print("\n4. Mining initial block...", Colors.BOLD)
        mine_result = run_cmd(f"go run cmd/miner/main.go -node localhost:8334 -privkey {privkey} -maxblocks 1", capture_output=False, timeout=15, output_color=Colors.BLUE)
        if mine_result is None or mine_result.returncode != 0:
            colored_print("Failed to mine initial block!", Colors.PURPLE)
            return
        colored_print("Mined initial block", Colors.BLUE)

        # Deploy a contract
        colored_print("\n5. Deploying contract...", Colors.BOLD)
        deploy_result = run_cmd(f"go run cmd/wallet/main.go --node localhost:8334 --privkey {privkey} deploy --amount 1000000 --gas-limit 50000 --gas-price 100", capture_output=False, output_color=Colors.RED)
        if deploy_result.returncode != 0:
            colored_print("Failed to deploy contract!", Colors.PURPLE)
            return
        colored_print("Contract deployed", Colors.RED)

        # Mine block with contract deployment
        colored_print("\n6. Mining block with contract deployment...", Colors.BOLD)
        mine_result2 = run_cmd(f"go run cmd/miner/main.go --node localhost:8334 --privkey {privkey} --maxblocks 1", capture_output=False, timeout=15, output_color=Colors.BLUE)
        if mine_result2 is None or mine_result2.returncode != 0:
            colored_print("Failed to mine block with contract!", Colors.PURPLE)
            colored_print("This is likely where the gas mismatch occurs!", Colors.PURPLE)
            return
        colored_print("Mined block with contract", Colors.BLUE)

        # Get contract address from node output (look for "Deployed contract at:")
        colored_print("\n7. Getting contract address...", Colors.BOLD)
        contract_addr = "3a789352b6c664552d62e37d3d9c78c02375626902ca3cecaeecdf6dccf18668"  # From deployment output
        colored_print(f"Using contract address: {contract_addr}", Colors.CYAN)

        # Check chain state before calling contract
        colored_print("\n8. Checking chain state before contract call...", Colors.BOLD)
        state_before = run_cmd(f"curl -s http://localhost:8334/chain-state | jq", output_color=Colors.CYAN)

        # Call the contract
        colored_print("\n9. Calling contract...", Colors.BOLD)
        call_result = run_cmd(f"go run cmd/wallet/main.go --node localhost:8334 --privkey {privkey} call --contract {contract_addr} --amount 0 --gas-limit 50000 --gas-price 100", capture_output=False, output_color=Colors.RED)
        if call_result.returncode != 0:
            colored_print("Contract call failed (expected for this demo)", Colors.RED)
        colored_print("Contract call attempted", Colors.RED)

        # Mine final block with contract call
        colored_print("\n10. Mining final block...", Colors.BOLD)
        mine_result3 = run_cmd(f"go run cmd/miner/main.go --node localhost:8334 --privkey {privkey} --maxblocks 1", capture_output=False, timeout=15, output_color=Colors.BLUE)
        if mine_result3 is None or mine_result3.returncode != 0:
            colored_print("Failed to mine final block!", Colors.PURPLE)
            colored_print("This might show the gas mismatch error:", Colors.PURPLE)
        else:
            colored_print("Successfully mined final block", Colors.BLUE)

        # Check chain state after calling contract
        colored_print("\n11. Checking chain state after contract call...", Colors.BOLD)
        state_after = run_cmd(f"curl -s http://localhost:8334/chain-state | jq", output_color=Colors.CYAN)

        # Parse and display contract storage changes
        colored_print("\n12. Analyzing contract storage changes...", Colors.BOLD)
        import json
        try:
            if state_before and state_before.stdout:
                before_data = json.loads(state_before.stdout)
                before_contract = before_data.get("AccountStates", {}).get(contract_addr, {})
                before_storage = before_contract.get("Persistent", {})
                colored_print(f"Storage before call: {before_storage}", Colors.YELLOW)

            if state_after and state_after.stdout:
                after_data = json.loads(state_after.stdout)
                after_contract = after_data.get("AccountStates", {}).get(contract_addr, {})
                after_storage = after_contract.get("Persistent", {})
                colored_print(f"Storage after call: {after_storage}", Colors.YELLOW)

                # Check if value changed from 42 to 43 (increment)
                if "1" in after_storage:
                    value = after_storage["1"]
                    colored_print(f"Contract storage at key '1': {value}", Colors.GREEN)
                    if value == "43":
                        colored_print("Contract successfully incremented value from 42 to 43!", Colors.GREEN)
                    else:
                        colored_print(f"Unexpected value: expected 43, got {value}", Colors.PURPLE)
        except Exception as e:
            colored_print(f"Error parsing chain state: {e}", Colors.PURPLE)

    finally:
        # Stop the output reading thread
        colored_print("\n13. Stopping seed node...", Colors.BOLD)
        stop_event.set()

        # Kill the seed node
        seed_process.terminate()
        try:
            seed_process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            seed_process.kill()

        # Wait for output thread to finish
        output_thread.join(timeout=2)
        colored_print("Seed node stopped", Colors.GREEN)

    print("\n=== Debug session complete ===")

if __name__ == "__main__":
    main()