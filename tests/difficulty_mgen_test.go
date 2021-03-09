// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tests

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params/vars"
)

var outNDJSONFile = filepath.Join(difficultyTestDir, "mgen_difficulty.ndjson")

func TestDifficultyGen(t *testing.T) {
	generateTests := os.Getenv(CG_GENERATE_DIFFICULTY_TESTS_KEY) != ""

	if !generateTests {
		t.Skip()
	}
	if os.Getenv(CG_CHAINCONFIG_CHAINSPECS_OPENETHEREUM_KEY) == "" {
		t.Fatal("Must run test generation with JSON file chain configurations.")
	}

	err := os.MkdirAll(filepath.Dir(outNDJSONFile), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	// Truncate/touch output file.
	err = ioutil.WriteFile(outNDJSONFile, []byte{}, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	dt := new(testMatcher)

	// Not difficulty-tests
	dt.skipLoad("hexencodetest.*")
	dt.skipLoad("crypto.*")
	dt.skipLoad("blockgenesistest\\.json")
	dt.skipLoad("genesishashestest\\.json")
	dt.skipLoad("keyaddrtest\\.json")
	dt.skipLoad("txtest\\.json")

	// files are 2 years old, contains strange values
	dt.skipLoad("difficultyCustomHomestead\\.json")
	dt.skipLoad("difficultyMorden\\.json")
	dt.skipLoad("difficultyOlimpic\\.json")
	dt.skipLoad("all_difficulty_tests\\.json")

	for k, v := range difficultyChainConfigurations {
		dt.config(k, v)
	}

	// Map will hold pairs of newConfigName: chainSpecrefs.
	// It will be used during writing of associated chainspec config files.
	// See comment below.
	wroteNewChainConfigs := make(map[string]chainspecRef)

	dt.walk(t, difficultyTestDir, func(t *testing.T, name string, test *DifficultyTest) {
		cfg, key := dt.findConfig(name)

		if test.ParentDifficulty.Cmp(vars.MinimumDifficulty) < 0 {
			t.Skip("difficulty below minimum")
			return
		}
		if err := dt.checkFailure(t, name, test.Run(cfg)); err != nil {
			t.Fatalf("failed to run difficulty test, err=%v", err)
		} else {

			// Collect all paired tests and originals.
			// The output file will yield ALL tests, not just newly-generated ones.
			specFile, ok := chainspecRefsDifficulty[key]
			if !ok {
				t.Fatalf("missing spec ref; key=%s file=%s", key, specFile)
			}
			test.Chainspec = specFile
			test.Name = strings.ReplaceAll(name, ".json", "")
			mustAppendTestToFile(t, test, outNDJSONFile)

			// Kind of ugly reverse lookup from file -> fork name.
			var forkName string
			for k, v := range mapForkNameChainspecFileDifficulty {
				if v == test.Chainspec.Filename {
					forkName = k
					break
				}
			}
			if forkName == "" {
				t.Fatal("missing fork/fileconf name", test, mapForkNameChainspecFileDifficulty)
			}

			// Is test(config) associated with a new test to be generated.
			associateForkName, ok := writeDifficultyTestsReferencePairs[forkName]
			if !ok {
				t.Logf("OK [existing,nonref] %v", test)
				return
			}

			conf, ok := difficultyChainConfigurations[associateForkName]
			if !ok {
				t.Fatalf("config association failed; no existing Go chain config found: %s", associateForkName)
			}

			// If associated chainspec file has not been written at least once, write it.
			// This ensures that for test generation, a chain spec file be written if it does not already exist.
			// This is because it is more likely that we will want to write the chain spec in Go, then have the
			// generator write the spec along with the tests to save the hurdle of manually building the chain spec
			// file first as a dependency for test generation.
			specref, done := wroteNewChainConfigs[associateForkName]
			if !done || specFile.Filename == "" {
				t.Logf("Writing chain spec file: %s", forkName)
				specFilepath, sum, err := writeDifficultyConfigFile(conf, associateForkName)
				if err != nil {
					t.Fatalf("could not write difficulty file, err=%v", err)
				}
				specref = chainspecRef{
					Filename: specFilepath,
					Sha1Sum:  sum[:],
				}
				wroteNewChainConfigs[associateForkName] = specref
			}

			newTest := &DifficultyTest{
				ParentTimestamp:    test.ParentTimestamp,
				ParentDifficulty:   test.ParentDifficulty,
				UncleHash:          test.UncleHash,
				CurrentTimestamp:   test.CurrentTimestamp,
				CurrentBlockNumber: test.CurrentBlockNumber,
				CurrentDifficulty: ethash.CalcDifficulty(conf, test.CurrentTimestamp, &types.Header{
					Difficulty: test.ParentDifficulty,
					Time:       test.ParentTimestamp,
					Number:     big.NewInt(int64(test.CurrentBlockNumber - 1)),
					UncleHash:  test.UncleHash,
				}),
				Chainspec: specref,
				Name:      strings.ReplaceAll(test.Name, forkName, associateForkName),
			}

			// "Dogfood".
			if err := newTest.Run(conf); err != nil {
				t.Fatal(err)
			}
			mustAppendTestToFile(t, newTest, outNDJSONFile)
			t.Logf("OK [generated] %v", newTest)

		}
	})
}

func mustAppendTestToFile(t *testing.T, test *DifficultyTest, filep string) {
	b, _ := json.Marshal(test)
	out := []byte{}
	buf := bytes.NewBuffer(out)
	err := json.Compact(buf, b)
	if err != nil {
		t.Fatal(err)
	}
	buf.Write([]byte("\n"))

	fi, err := os.OpenFile(filep, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		t.Fatal(err)
		return
	}
	_, err = fi.Write(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	err = fi.Close()
	if err != nil {
		t.Fatal(err)
	}
}
