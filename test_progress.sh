#!/bin/bash

echo "Testing DVOM Progress Bars"
echo "=========================="
echo

# Create a test volume if it doesn't exist
echo "Creating test volume..."
docker volume create dvom-test-volume 2>/dev/null || true

# Add some test data to the volume
echo "Adding test data to volume..."
docker run --rm -v dvom-test-volume:/data alpine sh -c "dd if=/dev/urandom of=/data/testfile bs=1M count=50 2>/dev/null"

echo
echo "1. Testing backup with progress bar (default):"
echo "----------------------------------------------"
./bin/dvom backup --volume=dvom-test-volume --name=test-backup-progress

echo
echo "2. Testing backup with quiet mode (no progress bar):"
echo "---------------------------------------------------"
./bin/dvom backup --volume=dvom-test-volume --name=test-backup-quiet --quiet

echo
echo "3. Testing restore with progress bar:"
echo "------------------------------------"
docker volume create dvom-restore-test 2>/dev/null || true
./bin/dvom restore --snapshot=test-backup-progress --target-volume=dvom-restore-test --force

echo
echo "4. Cleanup:"
echo "-----------"
./bin/dvom delete test-backup-progress --force
./bin/dvom delete test-backup-quiet --force
docker volume rm dvom-test-volume dvom-restore-test 2>/dev/null || true

echo
echo "Progress bar test completed!"