package molecule

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

type OffChainParticipantBuilder struct {
	pub_key              SEC1EncodedPubKey
	payment_script       Script
	unlock_script        Script
	payment_min_capacity Uint64
}

func (s *OffChainParticipantBuilder) Build() OffChainParticipant {
	b := new(bytes.Buffer)

	totalSize := HeaderSizeUint * (4 + 1)
	offsets := make([]uint32, 0, 4)

	offsets = append(offsets, totalSize)
	totalSize += uint32(len(s.pub_key.AsSlice()))
	offsets = append(offsets, totalSize)
	totalSize += uint32(len(s.payment_script.AsSlice()))
	offsets = append(offsets, totalSize)
	totalSize += uint32(len(s.unlock_script.AsSlice()))
	offsets = append(offsets, totalSize)
	totalSize += uint32(len(s.payment_min_capacity.AsSlice()))

	b.Write(packNumber(Number(totalSize)))

	for i := 0; i < len(offsets); i++ {
		b.Write(packNumber(Number(offsets[i])))
	}

	b.Write(s.pub_key.AsSlice())
	b.Write(s.payment_script.AsSlice())
	b.Write(s.unlock_script.AsSlice())
	b.Write(s.payment_min_capacity.AsSlice())
	return OffChainParticipant{inner: b.Bytes()}
}

func (s *OffChainParticipantBuilder) PubKey(v SEC1EncodedPubKey) *OffChainParticipantBuilder {
	s.pub_key = v
	return s
}

func (s *OffChainParticipantBuilder) PaymentScript(v Script) *OffChainParticipantBuilder {
	s.payment_script = v
	return s
}

func (s *OffChainParticipantBuilder) UnlockScript(v Script) *OffChainParticipantBuilder {
	s.unlock_script = v
	return s
}

func (s *OffChainParticipantBuilder) PaymentMinCapacity(v Uint64) *OffChainParticipantBuilder {
	s.payment_min_capacity = v
	return s
}

func NewOffChainParticipantBuilder() *OffChainParticipantBuilder {
	return &OffChainParticipantBuilder{pub_key: SEC1EncodedPubKeyDefault(), payment_script: ScriptDefault(), unlock_script: ScriptDefault(), payment_min_capacity: Uint64Default()}
}

type OffChainParticipant struct {
	inner []byte
}

func OffChainParticipantFromSliceUnchecked(slice []byte) *OffChainParticipant {
	return &OffChainParticipant{inner: slice}
}
func (s *OffChainParticipant) AsSlice() []byte {
	return s.inner
}

func OffChainParticipantDefault() OffChainParticipant {
	return *OffChainParticipantFromSliceUnchecked([]byte{167, 0, 0, 0, 20, 0, 0, 0, 53, 0, 0, 0, 106, 0, 0, 0, 159, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 53, 0, 0, 0, 16, 0, 0, 0, 48, 0, 0, 0, 49, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 53, 0, 0, 0, 16, 0, 0, 0, 48, 0, 0, 0, 49, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
}

func OffChainParticipantFromSlice(slice []byte, compatible bool) (*OffChainParticipant, error) {
	sliceLen := len(slice)
	if uint32(sliceLen) < HeaderSizeUint {
		errMsg := strings.Join([]string{"HeaderIsBroken", "OffChainParticipant", strconv.Itoa(int(sliceLen)), "<", strconv.Itoa(int(HeaderSizeUint))}, " ")
		return nil, errors.New(errMsg)
	}

	totalSize := unpackNumber(slice)
	if Number(sliceLen) != totalSize {
		errMsg := strings.Join([]string{"TotalSizeNotMatch", "OffChainParticipant", strconv.Itoa(int(sliceLen)), "!=", strconv.Itoa(int(totalSize))}, " ")
		return nil, errors.New(errMsg)
	}

	if uint32(sliceLen) < HeaderSizeUint*2 {
		errMsg := strings.Join([]string{"TotalSizeNotMatch", "OffChainParticipant", strconv.Itoa(int(sliceLen)), "<", strconv.Itoa(int(HeaderSizeUint * 2))}, " ")
		return nil, errors.New(errMsg)
	}

	offsetFirst := unpackNumber(slice[HeaderSizeUint:])
	if uint32(offsetFirst)%HeaderSizeUint != 0 || uint32(offsetFirst) < HeaderSizeUint*2 {
		errMsg := strings.Join([]string{"OffsetsNotMatch", "OffChainParticipant", strconv.Itoa(int(offsetFirst % 4)), "!= 0", strconv.Itoa(int(offsetFirst)), "<", strconv.Itoa(int(HeaderSizeUint * 2))}, " ")
		return nil, errors.New(errMsg)
	}

	if sliceLen < int(offsetFirst) {
		errMsg := strings.Join([]string{"HeaderIsBroken", "OffChainParticipant", strconv.Itoa(int(sliceLen)), "<", strconv.Itoa(int(offsetFirst))}, " ")
		return nil, errors.New(errMsg)
	}

	fieldCount := uint32(offsetFirst)/HeaderSizeUint - 1
	if fieldCount < 4 {
		return nil, errors.New("FieldCountNotMatch")
	} else if !compatible && fieldCount > 4 {
		return nil, errors.New("FieldCountNotMatch")
	}

	offsets := make([]uint32, fieldCount)

	for i := 0; i < int(fieldCount); i++ {
		offsets[i] = uint32(unpackNumber(slice[HeaderSizeUint:][int(HeaderSizeUint)*i:]))
	}
	offsets = append(offsets, uint32(totalSize))

	for i := 0; i < len(offsets); i++ {
		if i&1 != 0 && offsets[i-1] > offsets[i] {
			return nil, errors.New("OffsetsNotMatch")
		}
	}

	var err error

	_, err = SEC1EncodedPubKeyFromSlice(slice[offsets[0]:offsets[1]], compatible)
	if err != nil {
		return nil, err
	}

	_, err = ScriptFromSlice(slice[offsets[1]:offsets[2]], compatible)
	if err != nil {
		return nil, err
	}

	_, err = ScriptFromSlice(slice[offsets[2]:offsets[3]], compatible)
	if err != nil {
		return nil, err
	}

	_, err = Uint64FromSlice(slice[offsets[3]:offsets[4]], compatible)
	if err != nil {
		return nil, err
	}

	return &OffChainParticipant{inner: slice}, nil
}

func (s *OffChainParticipant) TotalSize() uint {
	return uint(unpackNumber(s.inner))
}
func (s *OffChainParticipant) FieldCount() uint {
	var number uint = 0
	if uint32(s.TotalSize()) == HeaderSizeUint {
		return number
	}
	number = uint(unpackNumber(s.inner[HeaderSizeUint:]))/4 - 1
	return number
}
func (s *OffChainParticipant) Len() uint {
	return s.FieldCount()
}
func (s *OffChainParticipant) IsEmpty() bool {
	return s.Len() == 0
}
func (s *OffChainParticipant) CountExtraFields() uint {
	return s.FieldCount() - 4
}

func (s *OffChainParticipant) HasExtraFields() bool {
	return 4 != s.FieldCount()
}

func (s *OffChainParticipant) PubKey() *SEC1EncodedPubKey {
	start := unpackNumber(s.inner[4:])
	end := unpackNumber(s.inner[8:])
	return SEC1EncodedPubKeyFromSliceUnchecked(s.inner[start:end])
}

func (s *OffChainParticipant) PaymentScript() *Script {
	start := unpackNumber(s.inner[8:])
	end := unpackNumber(s.inner[12:])
	return ScriptFromSliceUnchecked(s.inner[start:end])
}

func (s *OffChainParticipant) UnlockScript() *Script {
	start := unpackNumber(s.inner[12:])
	end := unpackNumber(s.inner[16:])
	return ScriptFromSliceUnchecked(s.inner[start:end])
}

func (s *OffChainParticipant) PaymentMinCapacity() *Uint64 {
	var ret *Uint64
	start := unpackNumber(s.inner[16:])
	if s.HasExtraFields() {
		end := unpackNumber(s.inner[20:])
		ret = Uint64FromSliceUnchecked(s.inner[start:end])
	} else {
		ret = Uint64FromSliceUnchecked(s.inner[start:])
	}
	return ret
}

func (s *OffChainParticipant) AsBuilder() OffChainParticipantBuilder {
	ret := NewOffChainParticipantBuilder().PubKey(*s.PubKey()).PaymentScript(*s.PaymentScript()).UnlockScript(*s.UnlockScript()).PaymentMinCapacity(*s.PaymentMinCapacity())
	return *ret
}
