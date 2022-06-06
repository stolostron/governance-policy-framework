#! /bin/bash

set -e

cd "$(dirname "$0")"

passes=0

#makefile, imports, and manifests all pass, no error expected
echo "Case 1: Makefile, imports, manifest do not have upstream references"
if (./../detect-upstream.sh -m "cat mock_makefile_pass.txt" -i "cat mock_deps_output_pass.txt" -q "mock_manifest_pass.yaml"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#makefile fails, imports pass, error expected
echo "Case 2: Makefile has upstream references"
if ! (./../detect-upstream.sh -m "cat mock_makefile_fail.txt" -i "cat mock_deps_output_pass.txt" -q "mock_manifest_pass.yaml"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#makefile passes, imports fail, error expected
echo "Case 3: Imports have upstream references"
if ! (./../detect-upstream.sh -m "cat mock_makefile_pass.txt" -i "cat mock_deps_output_fail.txt" -q "mock_manifest_pass.yaml"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#all checks fail, error expected
echo "Case 4: Everything has upstream references"
if ! (./../detect-upstream.sh -m "cat mock_makefile_fail.txt" -i "cat mock_deps_output_fail.txt" -q "mock_manifest_fail.yaml"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#manifests fail, error expected
echo "Case 5: Manifests have upstream references"
if ! (./../detect-upstream.sh -m "cat mock_makefile_pass.txt" -i "cat mock_deps_output_pass.txt" -q "mock_manifest_fail.yaml"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

echo ------------------------------------------------------
echo "Upstream script tests completed, ${passes}/5 passed"
if [[ $passes < 5 ]]; then
  exit 1
fi