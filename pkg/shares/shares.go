package shares

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/nmt/namespace"
)

// Share contains the raw share data (including namespace ID).
type Share struct {
	data []byte
}

func newShare(data []byte) (*Share, error) {
	if err := validateSize(data); err != nil {
		return nil, err
	}
	return &Share{data}, nil
}

func (s *Share) Validate() error {
	return validateSize(s.data)
}

func validateSize(data []byte) error {
	if len(data) != appconsts.ShareSize {
		return fmt.Errorf("share data must be %d bytes, got %d", appconsts.ShareSize, len(data))
	}
	return nil
}

func (s *Share) NamespaceID() namespace.ID {
	if len(s.data) < appconsts.NamespaceSize {
		panic(fmt.Sprintf("share %s is too short to contain a namespace ID", s))
	}
	return namespace.ID(s.data[:appconsts.NamespaceSize])
}

func (s *Share) Len() int {
	return len(s.data)
}

func (s *Share) InfoByte() (InfoByte, error) {
	if len(s.data) < appconsts.NamespaceSize+appconsts.ShareInfoBytes {
		return 0, fmt.Errorf("share %s is too short to contain an info byte", s)
	}
	// the info byte is the first byte after the namespace ID
	unparsed := s.data[appconsts.NamespaceSize]
	return ParseInfoByte(unparsed)
}

func (s *Share) Version() (uint8, error) {
	infoByte, err := s.InfoByte()
	if err != nil {
		return 0, err
	}
	return infoByte.Version(), nil
}

func (s *Share) DoesSupportVersions(supportedShareVersions []uint8) error {
	ver, err := s.Version()
	if err != nil {
		return err
	}
	if !bytes.Contains(supportedShareVersions, []byte{ver}) {
		return fmt.Errorf("unsupported share version %v is not present in the list of supported share versions %v", ver, supportedShareVersions)
	}
	return nil
}

// IsSequenceStart returns true if this is the first share in a sequence.
func (s *Share) IsSequenceStart() (bool, error) {
	infoByte, err := s.InfoByte()
	if err != nil {
		return false, err
	}
	return infoByte.IsSequenceStart(), nil
}

// IsCompactShare returns true if this is a compact share.
func (s *Share) IsCompactShare() bool {
	return s.NamespaceID().Equal(appconsts.TxNamespaceID) || s.NamespaceID().Equal(appconsts.PayForBlobNamespaceID)
}

// SequenceLen returns the sequence length of this *share and optionally an
// error. It returns 0, nil if this is a continuation share (i.e. doesn't
// contain a sequence length).
func (s *Share) SequenceLen() (sequenceLen uint32, err error) {
	isSequenceStart, err := s.IsSequenceStart()
	if err != nil {
		return 0, err
	}
	if !isSequenceStart {
		return 0, nil
	}

	start := appconsts.NamespaceSize + appconsts.ShareInfoBytes
	end := start + appconsts.SequenceLenBytes
	if len(s.data) < end {
		return 0, fmt.Errorf("share %s is too short to contain a sequence length", s)
	}
	return binary.BigEndian.Uint32(s.data[start:end]), nil
}

// IsPadding returns whether this *share is padding or not.
func (s *Share) IsPadding() (bool, error) {
	isNamespacePadding, err := s.isNamespacePadding()
	if err != nil {
		return false, err
	}
	return isNamespacePadding || s.isTailPadding() || s.isReservedPadding(), nil
}

func (s *Share) isNamespacePadding() (bool, error) {
	isSequenceStart, err := s.IsSequenceStart()
	if err != nil {
		return false, err
	}
	sequenceLen, err := s.SequenceLen()
	if err != nil {
		return false, err
	}

	return isSequenceStart && sequenceLen == 0, nil
}

func (s *Share) isTailPadding() bool {
	return s.NamespaceID().Equal(appconsts.TailPaddingNamespaceID)
}

func (s *Share) isReservedPadding() bool {
	return s.NamespaceID().Equal(appconsts.ReservedPaddingNamespaceID)
}

func (s *Share) ToBytes() []byte {
	return s.data
}

// RawData returns the raw share data. The raw share data does not contain the
// namespace ID, info byte, sequence length, or reserved bytes.
func (s *Share) RawData() (rawData []byte, err error) {
	if len(s.data) < s.rawDataStartIndex() {
		return rawData, fmt.Errorf("share %s is too short to contain raw data", s)
	}

	return s.data[s.rawDataStartIndex():], nil
}

func (s *Share) rawDataStartIndex() int {
	isStart, err := s.IsSequenceStart()
	if err != nil {
		panic(err)
	}
	if isStart && s.IsCompactShare() {
		return appconsts.NamespaceSize + appconsts.ShareInfoBytes + appconsts.SequenceLenBytes + appconsts.CompactShareReservedBytes
	} else if isStart && !s.IsCompactShare() {
		return appconsts.NamespaceSize + appconsts.ShareInfoBytes + appconsts.SequenceLenBytes
	} else if !isStart && s.IsCompactShare() {
		return appconsts.NamespaceSize + appconsts.ShareInfoBytes + appconsts.CompactShareReservedBytes
	} else if !isStart && !s.IsCompactShare() {
		return appconsts.NamespaceSize + appconsts.ShareInfoBytes
	} else {
		panic(fmt.Sprintf("unable to determine the rawDataStartIndex for share %s", s.data))
	}
}

func ToBytes(shares []Share) (bytes [][]byte) {
	bytes = make([][]byte, len(shares))
	for i, share := range shares {
		bytes[i] = []byte(share.data)
	}
	return bytes
}

func FromBytes(bytes [][]byte) (shares []Share) {
	shares = make([]Share, len(bytes))
	for i, b := range bytes {
		shares[i] = Share{data: b}
	}
	return shares
}
