#!/bin/bash

# TRUF CRE Complete Workflow Demo
# This script runs three separate workflows to demonstrate the complete CRUD lifecycle
# while staying within CRE simulation's 5 HTTP request limit per workflow

echo "========================================================"
echo "TRUF CRE Complete Workflow Demo (3-Part)"
echo "========================================================"
echo ""

# Step 1: Run write workflow (Deploy + Insert)
echo "[1/5] Running Write Workflow (Deploy + Insert)..."
echo "--------------------------------------------------------"
cre workflow simulate truf-write-workflow

if [ $? -ne 0 ]; then
    echo "❌ Write workflow failed!"
    exit 1
fi

echo ""
echo "[2/5] Waiting 5 seconds for transactions to confirm..."
echo "--------------------------------------------------------"
sleep 5

# Step 2: Run read workflow (Get Records)
echo ""
echo "[3/5] Running Read Workflow (Get Records)..."
echo "--------------------------------------------------------"
cre workflow simulate truf-read-workflow

if [ $? -ne 0 ]; then
    echo "❌ Read workflow failed!"
    exit 1
fi

echo ""
echo "[4/5] Waiting 5 seconds before cleanup..."
echo "--------------------------------------------------------"
sleep 5

# Step 3: Run cleanup workflow (Delete Stream)
echo ""
echo "[5/5] Running Cleanup Workflow (Delete Stream)..."
echo "--------------------------------------------------------"
cre workflow simulate truf-cleanup-workflow

if [ $? -ne 0 ]; then
    echo "❌ Cleanup workflow failed!"
    exit 1
fi

echo ""
echo "========================================================"
echo "✅ Complete Workflow Demo Finished Successfully!"
echo "========================================================"
echo ""
echo "Summary:"
echo "  ✓ Workflow 1: Stream deployed + Records inserted"
echo "  ✓ Workflow 2: Records retrieved"
echo "  ✓ Workflow 3: Stream deleted (cleanup)"
echo ""
echo "Complete CRUD lifecycle demonstrated across 3 workflows!"
echo ""
