package rugaroo

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestFuzzDiff is decision 436: each testdata/fuzz/<area>.rb driver is compiled once, fed the same generated cases as `ruby`, and the outputs must agree line for line.
// Plain `go test` replays testdata/fuzz/corpus/<area>.txt only; RUGAROO_FUZZ_DIFF=1 adds RUGAROO_FUZZ_N (default 20000) cases from RUGAROO_FUZZ_SEED (default 1).
func TestFuzzDiff(t *testing.T) {
	t.Parallel()
	gens := map[string]func(*rand.Rand) string{"fmt": genFmtCase}
	for area, gen := range gens {
		t.Run(area, func(t *testing.T) {
			t.Parallel()
			cases := corpusCases(t, area)
			if os.Getenv("RUGAROO_FUZZ_DIFF") != "" {
				seed, n := envInt("RUGAROO_FUZZ_SEED", 1), envInt("RUGAROO_FUZZ_N", 20000)
				r := rand.New(rand.NewPCG(uint64(seed), 0)) //nolint:gosec // test data
				for range n {
					cases = append(cases, gen(r))
				}
			}
			if len(cases) == 0 {
				t.Skip("no corpus; RUGAROO_FUZZ_DIFF=1 generates cases")
			}
			requireToolchain(t)
			rb := filepath.Join("testdata", "fuzz", area+".rb")
			src, err := os.ReadFile(rb) //nolint:gosec // testdata path
			if err != nil {
				t.Fatal(err)
			}
			bin := goBuild(t, transpile(t, rb, src), "-l")
			in := strings.Join(cases, "\n") + "\n"
			want, _, _, err := runIO(t, filepath.Dir(rb), progIO{stdin: in}, "ruby", filepath.Base(rb))
			if err != nil {
				t.Fatal(err)
			}
			got, stderr, _, _ := runIO(t, filepath.Dir(rb), progIO{stdin: in, env: []string{"RUGAROO_GO_STACK=1"}}, bin)
			diffCases(t, cases, strings.Split(strings.TrimSuffix(want, "\n"), "\n"), strings.Split(strings.TrimSuffix(got, "\n"), "\n"), stderr)
		})
	}
}

func diffCases(t *testing.T, cases, want, got []string, stderr string) {
	t.Helper()
	bad := 0
	byOp := map[string]int{}
	for i, c := range cases {
		w := ""
		if i < len(want) {
			w = want[i]
		}
		g := "<missing: program stopped>"
		if i < len(got) {
			g = got[i]
		}
		if w == g {
			continue
		}
		if dump := os.Getenv("RUGAROO_FUZZ_DUMP"); dump != "" { // every mismatch as case<TAB>mri<TAB>got, for bucketing by root cause
			f, _ := os.OpenFile(dump, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // developer-chosen path
			fmt.Fprintf(f, "%s\t|\t%s\t|\t%s\n", c, w, g)
			f.Close()
		}
		byOp[strings.SplitN(c, "\t", 2)[0]]++
		if bad++; bad <= 20 {
			t.Errorf("case %s\n  mri: %s\n  got: %s\n  decoded: %s", c, w, g, decodeCase(c))
		}
		if i >= len(got) {
			t.Errorf("program output ended at case %d; stderr:\n%s", i, stderr)
			break
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d cases differ %v; add the lines worth keeping to testdata/fuzz/corpus/", bad, len(cases), byOp)
	}
}

func decodeCase(c string) string {
	parts := strings.Split(c, "\t")
	for i, p := range parts[1:] {
		if b, err := hex.DecodeString(p); err == nil && len(p)%2 == 0 {
			parts[i+1] = strconv.Quote(string(b))
		}
	}
	return strings.Join(parts, " ")
}

func corpusCases(t *testing.T, area string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "fuzz", "corpus", area+".txt")) //nolint:gosec // testdata path
	if err != nil {
		return nil
	}
	var out []string
	for l := range strings.SplitSeq(string(b), "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return n
	}
	return def
}

func hx(s string) string { return hex.EncodeToString([]byte(s)) }

func pick[T any](r *rand.Rand, xs ...T) T { return xs[r.IntN(len(xs))] }

func fuzzFloat(r *rand.Rand) string {
	var f float64
	switch r.IntN(7) {
	case 0:
		f = pick(r, 0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), 1, -1, 0.5, 2.5, 1e22, 1e23, 5e-324, math.MaxFloat64, 9007199254740993, 0.1, 0.3, 1.005, 2.675)
	case 1:
		f = math.Float64frombits(r.Uint64())
	case 2:
		f = float64(r.IntN(2000)-1000) / math.Pow(10, float64(r.IntN(8)))
	case 3:
		f = float64(r.IntN(200)-100) + 0.5
	case 4:
		f = float64(r.Int64N(1<<53)) * math.Pow(2, float64(r.IntN(120)-60))
	case 5:
		f = (r.Float64() - 0.5) * math.Pow(10, float64(r.IntN(40)-20))
	default:
		f = math.Float64frombits(r.Uint64() &^ (0x7ff << 52)) // subnormal
	}
	return floatHex(f)
}

// floatHex is f's big-endian IEEE bits, the driver's float encoding.
func floatHex(f float64) string {
	var b [8]byte
	bits := math.Float64bits(f)
	for i := range b {
		b[i] = byte(bits >> (56 - 8*i))
	}
	return hex.EncodeToString(b[:])
}

func fuzzInt(r *rand.Rand) string {
	switch r.IntN(4) {
	case 0:
		return strconv.FormatInt(pick[int64](r, 0, 1, -1, 1<<62, -(1<<62), math.MaxInt64, math.MinInt64, 255, 256, -255), 10)
	case 1:
		return strconv.FormatInt(r.Int64(), 10)
	default:
		return strconv.Itoa(r.IntN(2000) - 1000)
	}
}

// fuzzString draws from ASCII, multibyte, escapes and invalid UTF-8.
func fuzzString(r *rand.Rand, alphabet []string) string {
	var b strings.Builder
	for range r.IntN(10) {
		b.WriteString(pick(r, alphabet...))
	}
	return b.String()
}

var strAlphabet = []string{"a", "b", "z", "Z", "9", " ", "-", "_", "\n", "é", "日", "\xff", "\xc3", "0x", "1_0", "+", ".", "e", "0b1", "  ", "\x00", "zz", "Az"}

// validAlphabet is strAlphabet without invalid UTF-8: MRI raises on a broken string in tr/count/split, and rugaroo strings are valid UTF-8 (decision 380 area, not a set-parser bug).
var validAlphabet = []string{"a", "b", "z", "Z", "9", " ", "-", "_", "\n", "é", "日", "0x", "1_0", "+", ".", "e", "0b1", "  ", "\x00", "zz", "Az"}

var asciiAlphabet = []string{"a", "b", "z", "Z", "9", "0", "1", " ", "-", "_", "\n", ".", "A", "zz", "99", "\x00"}
var setAlphabet = []string{"a", "z", "-", "^", "\\", "a-z", "A-Z", "0-9", "a-c", "x-b", "é", "\\-", "\\^", "\\\\", "^a", "日", "z-a"}

func fuzzFmtSpec(r *rand.Rand, convs string) string {
	if r.IntN(12) == 0 { // malformed: random directive characters
		return fuzzString(r, []string{"%", "-", "+", " ", "0", "#", "*", ".", "1", "5", "$", "<a>", "{a}", "d", "s", "x", "%%", "l"})
	}
	var b strings.Builder
	b.WriteString(pick(r, "", "x", "%% ", "v="))
	b.WriteByte('%')
	for _, f := range "-+ 0#" {
		if r.IntN(4) == 0 {
			b.WriteRune(f)
		}
	}
	if r.IntN(2) == 0 {
		b.WriteString(strconv.Itoa(r.IntN(30)))
	}
	if r.IntN(2) == 0 {
		b.WriteString("." + strconv.Itoa(r.IntN(30)))
	}
	b.WriteByte(convs[r.IntN(len(convs))])
	return b.String()
}

const packDirs = "aAZBbHhcCsSlLqQjJnNvVUwmMuDdFfEeGgxX@"

func fuzzPackFmt(r *rand.Rand) string {
	var b strings.Builder
	for range 1 + r.IntN(4) {
		b.WriteByte(packDirs[r.IntN(len(packDirs))])
		if r.IntN(5) == 0 {
			b.WriteString(pick(r, "_", "!", "<", ">"))
		}
		switch r.IntN(4) {
		case 0:
			b.WriteString("*")
		case 1:
			b.WriteString(strconv.Itoa(r.IntN(10)))
		}
	}
	return b.String()
}

func genFmtCase(r *rand.Rand) string {
	line := func(op string, args ...string) string { return op + "\t" + strings.Join(args, "\t") }
	switch r.IntN(19) {
	case 0, 1, 2:
		spec := fuzzFmtSpec(r, "eEfgGaAdsi")
		f := fuzzFloat(r)
		if strings.ContainsAny(spec, "dixXobBuc") || strings.ContainsAny(spec, "eE") && f[:3] == "000" { // past 64 bits MRI answers a Bignum (decision 35); MRI's %e of a subnormal rounds its 13th digit wrongly
			f = floatHex(float64(r.Int64N(1<<40)-1<<39) / float64(1+r.IntN(1000)))
		}
		return line("fmt_float", hx(spec), f)
	case 3, 4:
		return line("fmt_int", hx(fuzzFmtSpec(r, "dixXobBcsuf")), fuzzInt(r))
	case 5:
		return line("fmt_str", hx(fuzzFmtSpec(r, "scpd")), hx(fuzzString(r, validAlphabet)))
	case 6:
		return line("float_to_s", fuzzFloat(r))
	case 7:
		return line(pick(r, "float_round", "float_floor", "float_ceil"), fuzzFloat(r), strconv.Itoa(1+r.IntN(24))) // digits <= 0 make MRI answer an Integer, which a computed digits cannot (decision 299)
	case 8, 9:
		ints := make([]string, r.IntN(5))
		for i := range ints {
			ints[i] = fuzzInt(r)
		}
		return line("pack_int", hx(fuzzPackFmt(r)), strings.Join(ints, ","))
	case 10, 11:
		raw := make([]byte, r.IntN(12))
		for i := range raw {
			raw[i] = byte(r.IntN(256))
		}
		return line("unpack", hx(fuzzPackFmt(r)), hex.EncodeToString(raw))
	case 12:
		return line(pick(r, "str_tr"), hx(fuzzString(r, validAlphabet)), hx(fuzzString(r, setAlphabet)), hx(fuzzString(r, setAlphabet)))
	case 13:
		return line(pick(r, "str_delete", "str_squeeze", "str_count"), hx(fuzzString(r, validAlphabet)), hx(fuzzString(r, setAlphabet)))
	case 14:
		return line("str_succ", hx(fuzzString(r, asciiAlphabet))) // rbStrSucc is ASCII only: MRI also steps é to ê and 日 to 旦
	case 15:
		return line("str_to_i", hx(fuzzString(r, strAlphabet)), strconv.Itoa(2+r.IntN(35)))
	case 16:
		return line("str_integer", hx(fuzzString(r, strAlphabet)), strconv.Itoa(pick(r, 2, 8, 10, 16, 36, 0)))
	case 17:
		return line("str_float", hx(fuzzString(r, strAlphabet)))
	default:
		if r.IntN(2) == 0 {
			return line("str_split", hx(fuzzString(r, validAlphabet)), hx(pick(r, " ", "", "a", "-", "é")), strconv.Itoa(r.IntN(6)-2))
		}
		return line("str_slice", hx(fuzzString(r, validAlphabet)), strconv.Itoa(r.IntN(14)-7), strconv.Itoa(r.IntN(10)-3))
	}
}
