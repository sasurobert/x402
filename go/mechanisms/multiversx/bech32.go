package multiversx

import (
	"fmt"
	"strings"
)

const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var gen = []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}

func bech32Polymod(values []int) int {
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ v
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32HrpExpand(hrp string) []int {
	v := make([]int, 0, len(hrp)*2+1)
	for i := 0; i < len(hrp); i++ {
		v = append(v, int(hrp[i]>>5))
	}
	v = append(v, 0)
	for i := 0; i < len(hrp); i++ {
		v = append(v, int(hrp[i]&31))
	}
	return v
}

func bech32VerifyChecksum(hrp string, data []int) bool {
	return bech32Polymod(append(bech32HrpExpand(hrp), data...)) == 1
}

// EncodeBech32 encodes a byte slice into a bech32 string with the given HRP
func EncodeBech32(hrp string, data []byte) (string, error) {
	// Convert 8-bit to 5-bit
	converted, err := convertBits(intSlice(data), 8, 5, true)
	if err != nil {
		return "", err
	}

	return bech32Encode(hrp, converted), nil
}

func bech32Encode(hrp string, data []byte) string {
	combined := append(bech32HrpExpand(hrp), intSlice(data)...)
	checksum := bech32CreateChecksum(hrp, intSlice(data))
	combined = append(combined, checksum...)

	var sb strings.Builder
	sb.WriteString(hrp)
	sb.WriteString("1")
	// The combined part is used for checksum, but string encoding uses 5-bit mapped to charset
	// The data+checksum part needs to be mapped.
	// We need 5-bit values for data+checksum.
	// intSlice(data) is already 5-bit hopefully? No, EncodeBech32 converts it.
	// But bech32Encode receives 'data' which is 'converted'. Yes.

	values := intSlice(data)
	// Append checksum to values
	// wait, checksum is calculated using hrp+values.
	// The output string is hrp + '1' + mapped(values) + mapped(checksum)

	// Re-calculating checksum here just to be safe it flows correctly from `EncodeBech32`
	// but `bech32Encode` signature is `data []byte`.

	for _, v := range values {
		sb.WriteByte(charset[v])
	}
	for _, v := range checksum {
		sb.WriteByte(charset[v])
	}
	return sb.String()
}

// DecodeBech32 decodes a bech32 string
func DecodeBech32(bech string) (string, []byte, error) {
	if len(bech) < 8 || len(bech) > 90 {
		return "", nil, fmt.Errorf("invalid bech32 string length")
	}

	if strings.ToLower(bech) != bech && strings.ToUpper(bech) != bech {
		// Mixed case invalid
	}
	bechLower := strings.ToLower(bech)

	one := strings.LastIndex(bechLower, "1")
	if one < 1 || one+7 > len(bechLower) {
		return "", nil, fmt.Errorf("invalid index of 1")
	}

	hrp := bechLower[:one]
	data := bechLower[one+1:]

	var dataInts []int
	for i := 0; i < len(data); i++ {
		idx := strings.IndexByte(charset, data[i])
		if idx == -1 {
			return "", nil, fmt.Errorf("invalid character in data part: %c", data[i])
		}
		dataInts = append(dataInts, idx)
	}

	if !bech32VerifyChecksum(hrp, dataInts) {
		return "", nil, fmt.Errorf("invalid checksum")
	}

	dataInts = dataInts[:len(dataInts)-6]

	decoded, err := convertBits(dataInts, 5, 8, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert bits: %v", err)
	}

	return hrp, decoded, nil
}

func bech32CreateChecksum(hrp string, data []int) []int {
	values := append(bech32HrpExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	mod := bech32Polymod(values) ^ 1
	ret := make([]int, 6)
	for p := 0; p < 6; p++ {
		ret[p] = (mod >> uint(5*(5-p))) & 31
	}
	return ret
}

func convertBits(data []int, fromBits int, toBits int, pad bool) ([]byte, error) {
	acc := 0
	bits := 0
	out := make([]byte, 0)
	maxv := (1 << toBits) - 1
	max_acc := (1 << (fromBits + toBits - 1)) - 1

	for _, v := range data {
		if v < 0 || (v>>fromBits) != 0 {
			return nil, fmt.Errorf("invalid value")
		}
		acc = ((acc << fromBits) | v) & max_acc
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			out = append(out, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			out = append(out, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	return out, nil
}

func intSlice(data []byte) []int {
	out := make([]int, len(data))
	for i, b := range data {
		out[i] = int(b)
	}
	return out
}
