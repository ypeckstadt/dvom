#!/bin/bash

echo "Testing DVOM Encryption Support"
echo "================================"
echo

# Create a test volume if it doesn't exist
echo "Creating test volume..."
docker volume create dvom-encrypt-test 2>/dev/null || true

# Add some test data to the volume
echo "Adding test data to volume..."
docker run --rm -v dvom-encrypt-test:/data alpine sh -c "echo 'Secret data for encryption test' > /data/secret.txt && echo 'More data here' > /data/test.txt"

echo
echo "1. Testing encrypted backup with password flag:"
echo "----------------------------------------------"
./bin/dvom backup --volume=dvom-encrypt-test --name=encrypted-backup --encrypt --password=testpass123

echo
echo "2. Testing list command (should show encryption status):"
echo "-------------------------------------------------------"
./bin/dvom list

echo
echo "3. Testing restore of encrypted backup:"
echo "--------------------------------------"
docker volume create dvom-restore-encrypted 2>/dev/null || true
./bin/dvom restore --snapshot=encrypted-backup --target-volume=dvom-restore-encrypted --password=testpass123 --force

echo
echo "4. Verifying restored data:"
echo "---------------------------"
docker run --rm -v dvom-restore-encrypted:/data alpine cat /data/secret.txt

echo
echo "5. Testing non-encrypted backup (for comparison):"
echo "------------------------------------------------"
./bin/dvom backup --volume=dvom-encrypt-test --name=plain-backup

echo
echo "6. List both backups to compare:"
echo "-------------------------------"
./bin/dvom list

echo
echo "7. Cleanup:"
echo "-----------"
./bin/dvom delete encrypted-backup --force
./bin/dvom delete plain-backup --force
docker volume rm dvom-encrypt-test dvom-restore-encrypted 2>/dev/null || true

echo
echo "Encryption test completed!"