package wire

import (
	"bytes"
	"io"

	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/qerr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StreamFrame", func() {
	const (
		versionGQUICStreamFrame = versionBigEndian
		versionIETFStreamFrame  = protocol.Version41
	)

	BeforeEach(func() {
		Expect(versionGQUICStreamFrame.UsesIETFStreamFrame()).To(BeFalse())
		Expect(versionIETFStreamFrame.UsesIETFStreamFrame()).To(BeTrue())
	})

	Context("when parsing", func() {
		Context("in big endian", func() {
			It("accepts frames with data length", func() {
				b := bytes.NewReader([]byte{0x80 ^ 0x20,
					0x1,      // stream id
					0x0, 0x6, // data length
					'f', 'o', 'o', 'b', 'a', 'r',
					'f', 'o', 'o', // additional bytes, not belonging to the STREAM frame
				})
				frame, err := ParseStreamFrame(b, versionBigEndian)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.FinBit).To(BeFalse())
				Expect(frame.StreamID).To(Equal(protocol.StreamID(1)))
				Expect(frame.DataLenPresent).To(BeTrue())
				Expect(frame.Data).To(Equal([]byte("foobar")))
				Expect(b.Len()).To(Equal(3)) // 3 additional bytes
			})

			It("accepts a frame with the FinBit set", func() {
				b := bytes.NewReader([]byte{0x80 ^ 0x40,
					0x1, // stream id
					'f', 'o', 'o', 'b', 'a', 'r',
				})
				frame, err := ParseStreamFrame(b, versionGQUICStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.FinBit).To(BeTrue())
				Expect(frame.DataLenPresent).To(BeFalse())
				Expect(frame.Data).To(Equal([]byte("foobar")))
				Expect(b.Len()).To(BeZero())
			})

			It("accepts an empty frame with FinBit set, with data length set", func() {
				// the STREAM frame, plus 3 additional bytes, not belonging to this frame
				b := bytes.NewReader([]byte{0x80 ^ 0x40 ^ 0x20,
					0x1,  // stream id
					0, 0, // data length
					'f', 'o', 'o', // additional bytes
				})
				frame, err := ParseStreamFrame(b, versionGQUICStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.FinBit).To(BeTrue())
				Expect(frame.DataLenPresent).To(BeTrue())
				Expect(frame.Data).To(BeEmpty())
				Expect(b.Len()).To(Equal(3))
			})

			It("errors on empty stream frames that don't have the FinBit set", func() {
				b := bytes.NewReader([]byte{0x80 ^ 0x20,
					0x1,  // stream id
					0, 0, // data length
				})
				_, err := ParseStreamFrame(b, versionGQUICStreamFrame)
				Expect(err).To(MatchError(qerr.EmptyStreamFrameNoFin))
			})

			It("rejects frames to too large dataLen", func() {
				b := bytes.NewReader([]byte{0x80 ^ 0x20,
					0x1,        // stream id
					0xff, 0xff, // data length
				})
				_, err := ParseStreamFrame(b, versionGQUICStreamFrame)
				Expect(err).To(MatchError(io.EOF))
			})

			It("rejects frames that overflow the offset", func() {
				// Offset + len(Data) overflows MaxByteCount
				f := &StreamFrame{
					StreamID: 1,
					Offset:   protocol.MaxByteCount,
					Data:     []byte{'f'},
				}
				b := &bytes.Buffer{}
				err := f.Write(b, versionGQUICStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				_, err = ParseStreamFrame(bytes.NewReader(b.Bytes()), versionGQUICStreamFrame)
				Expect(err).To(MatchError(qerr.Error(qerr.InvalidStreamData, "data overflows maximum offset")))
			})

			It("errors on EOFs", func() {
				data := []byte{0x80 ^ 0x20 ^ 0x4,
					0x1,       // stream id
					0x0, 0x2a, // offset
					0x0, 0x6, // data length,
					'f', 'o', 'o', 'b', 'a', 'r',
				}
				_, err := ParseStreamFrame(bytes.NewReader(data), versionBigEndian)
				Expect(err).NotTo(HaveOccurred())
				for i := range data {
					_, err := ParseStreamFrame(bytes.NewReader(data[0:i]), versionBigEndian)
					Expect(err).To(HaveOccurred())
				}
			})

			Context("Stream ID length", func() {
				It("parses a frame with a 1 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0x80,
						0x12, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.StreamID).To(Equal(protocol.StreamID(0x12)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 2 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x1,
						0xca, 0xfe, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.StreamID).To(Equal(protocol.StreamID(0xcafe)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 3 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x2,
						0x12, 0x34, 0x56, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.StreamID).To(Equal(protocol.StreamID(0x123456)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 3 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x3,
						0xde, 0xca, 0xfb, 0xad, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.StreamID).To(Equal(protocol.StreamID(0xdecafbad)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})
			})

			Context("Offset length", func() {
				It("parses a frame with an offset of 0", func() {
					b := bytes.NewReader([]byte{0x80,
						0x1, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(BeZero())
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 2 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x4,
						0x1,        // stream id
						0xca, 0xfe, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xcafe)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 3 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x8,
						0x1,              // stream id
						0xca, 0xfe, 0x99, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xcafe99)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 4 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x8 ^ 0x4,
						0x1,                    // stream id
						0xde, 0xad, 0xbe, 0xef, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xdeadbeef)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 5 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x10,
						0x1,                          // stream id
						0xde, 0xad, 0xbe, 0xef, 0x13, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xdeadbeef13)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 6 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x10 ^ 0x4,
						0x1,                                // stream id
						0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xdeadbeefcafe)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 7 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x10 ^ 0x8,
						0x1,                                      // stream id
						0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0x99, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xdeadbeefcafe99)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 8 byte offset", func() {
					b := bytes.NewReader([]byte{0x80 ^ 0x10 ^ 0x8 ^ 0x4,
						0x1,                                            // stream id
						0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0x13, 0x37, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					frame, err := ParseStreamFrame(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(frame.Offset).To(Equal(protocol.ByteCount(0xdeadbeefcafe1337)))
					Expect(frame.Data).To(Equal([]byte("foobar")))
				})
			})
		})

		Context("for the IETF format", func() {
			It("accepts frames with data length", func() {
				b := bytes.NewReader([]byte{0xc0 ^ 0x1,
					0x1,      // stream id
					0x0, 0x6, // data length
					'f', 'o', 'o', 'b', 'a', 'r',
					'f', 'o', 'o', // additional bytes, not belonging to the STREAM frame
				})
				frame, err := ParseStreamFrame(b, versionIETFStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.FinBit).To(BeFalse())
				Expect(frame.StreamID).To(Equal(protocol.StreamID(1)))
				Expect(frame.DataLenPresent).To(BeTrue())
				Expect(frame.Data).To(Equal([]byte("foobar")))
				Expect(b.Len()).To(Equal(3)) // 3 additional bytes
			})

			It("accepts a frame with the FinBit set", func() {
				b := bytes.NewReader([]byte{0xc0 ^ 0x20,
					0x1, // stream id
					'f', 'o', 'o', 'b', 'a', 'r',
				})
				frame, err := ParseStreamFrame(b, versionIETFStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.FinBit).To(BeTrue())
				Expect(frame.DataLenPresent).To(BeFalse())
				Expect(frame.Data).To(Equal([]byte("foobar")))
				Expect(b.Len()).To(BeZero())
			})

			It("accepts an empty frame with FinBit set, with data length set", func() {
				// the STREAM frame, plus 3 additional bytes, not belonging to this frame
				b := bytes.NewReader([]byte{0xc0 ^ 0x20 ^ 0x1,
					0x1,  // stream id
					0, 0, // data length
					'f', 'o', 'o', // additional bytes
				})
				frame, err := ParseStreamFrame(b, versionIETFStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.FinBit).To(BeTrue())
				Expect(frame.DataLenPresent).To(BeTrue())
				Expect(frame.Data).To(BeEmpty())
				Expect(b.Len()).To(Equal(3))
			})

			It("errors on empty stream frames that don't have the FinBit set", func() {
				b := bytes.NewReader([]byte{0xc0 ^ 0x1,
					0x1,  // stream id
					0, 0, // data length
				})
				_, err := ParseStreamFrame(b, versionIETFStreamFrame)
				Expect(err).To(MatchError(qerr.EmptyStreamFrameNoFin))
			})

			It("rejects frames to too large dataLen", func() {
				b := bytes.NewReader([]byte{0xc0 ^ 0x1,
					0x1,        // stream id
					0xff, 0xff, // data length
				})
				_, err := ParseStreamFrame(b, versionIETFStreamFrame)
				Expect(err).To(MatchError(io.EOF))
			})

			It("rejects frames that overflow the offset", func() {
				// Offset + len(Data) overflows MaxByteCount
				f := &StreamFrame{
					StreamID: 1,
					Offset:   protocol.MaxByteCount,
					Data:     []byte{'f'},
				}
				b := &bytes.Buffer{}
				err := f.Write(b, versionIETFStreamFrame)
				Expect(err).ToNot(HaveOccurred())
				_, err = ParseStreamFrame(bytes.NewReader(b.Bytes()), versionIETFStreamFrame)
				Expect(err).To(MatchError(qerr.Error(qerr.InvalidStreamData, "data overflows maximum offset")))
			})

			It("errors on EOFs", func() {
				data := []byte{0x80 ^ 0x20 ^ 0x4,
					0x1,       // stream id
					0x0, 0x2a, // offset
					0x0, 0x6, // data length,
					'f', 'o', 'o', 'b', 'a', 'r',
				}
				_, err := ParseStreamFrame(bytes.NewReader(data), versionBigEndian)
				Expect(err).NotTo(HaveOccurred())
				for i := range data {
					_, err := ParseStreamFrame(bytes.NewReader(data[0:i]), versionBigEndian)
					Expect(err).To(HaveOccurred())
				}
			})

			Context("Stream ID length", func() {
				It("parses a frame with a 1 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0xc0,
						0x99, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0x99)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 2 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0xc0 ^ 0x8,
						0xca, 0xfe, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0xcafe)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 3 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0xc0 ^ 0x10,
						0xc0, 0x00, 0x01, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0xc00001)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 4 byte Stream ID", func() {
					b := bytes.NewReader([]byte{0xc0 ^ 0x18,
						0xde, 0xca, 0xfb, 0xad, // stream id
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0xdecafbad)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})
			})

			Context("Offset length", func() {
				It("parses a frame with an offset of 0", func() {
					b := bytes.NewReader([]byte{0xc0,
						0x1, // stream ID
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0x1)))
					Expect(f.Offset).To(BeZero())
					Expect(f.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 2 byte offset", func() {
					b := bytes.NewReader([]byte{0xc0 ^ 0x2,
						0x1,        // stream ID
						0xca, 0xfe, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0x1)))
					Expect(f.Offset).To(Equal(protocol.ByteCount(0xcafe)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 4 byte offset", func() {
					b := bytes.NewReader([]byte{0xc0 ^ 0x4,
						0x1,                    // stream ID
						0xde, 0xad, 0xbe, 0xef, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0x1)))
					Expect(f.Offset).To(Equal(protocol.ByteCount(0xdeadbeef)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})

				It("parses a frame with a 4 byte offset", func() {
					b := bytes.NewReader([]byte{0xc0 ^ 0x6,
						0x1,                                            // stream ID
						0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0x13, 0x37, // offset
						'f', 'o', 'o', 'b', 'a', 'r',
					})
					f, err := ParseStreamFrame(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(f.StreamID).To(Equal(protocol.StreamID(0x1)))
					Expect(f.Offset).To(Equal(protocol.ByteCount(0xdeadbeefcafe1337)))
					Expect(f.Data).To(Equal([]byte("foobar")))
				})
			})
		})
	})

	Context("when writing", func() {
		Context("in big endian", func() {
			It("writes sample frame", func() {
				b := &bytes.Buffer{}
				err := (&StreamFrame{
					StreamID:       1,
					Data:           []byte("foobar"),
					DataLenPresent: true,
				}).Write(b, versionBigEndian)
				Expect(err).ToNot(HaveOccurred())
				Expect(b.Bytes()).To(Equal([]byte{0x80 ^ 0x20,
					0x1,      // stream id
					0x0, 0x6, // data length
					'f', 'o', 'o', 'b', 'a', 'r',
				}))
			})
		})

		It("sets the FinBit", func() {
			b := &bytes.Buffer{}
			err := (&StreamFrame{
				StreamID: 1,
				Data:     []byte("foobar"),
				FinBit:   true,
			}).Write(b, protocol.VersionWhatever)
			Expect(err).ToNot(HaveOccurred())
			Expect(b.Bytes()[0] & 0x40).To(Equal(byte(0x40)))
		})

		It("errors when length is zero and FIN is not set", func() {
			b := &bytes.Buffer{}
			err := (&StreamFrame{
				StreamID: 1,
			}).Write(b, protocol.VersionWhatever)
			Expect(err).To(MatchError("StreamFrame: attempting to write empty frame without FIN"))
		})

		It("has proper min length for a short StreamID and a short offset", func() {
			b := &bytes.Buffer{}
			f := &StreamFrame{
				StreamID: 1,
				Data:     []byte{},
				Offset:   0,
				FinBit:   true,
			}
			err := f.Write(b, protocol.VersionWhatever)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.MinLength(0)).To(Equal(protocol.ByteCount(b.Len())))
		})

		It("has proper min length for a long StreamID and a big offset", func() {
			b := &bytes.Buffer{}
			f := &StreamFrame{
				StreamID: 0xdecafbad,
				Data:     []byte{},
				Offset:   0xdeadbeefcafe,
				FinBit:   true,
			}
			err := f.Write(b, protocol.VersionWhatever)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.MinLength(0)).To(Equal(protocol.ByteCount(b.Len())))
		})

		Context("data length field", func() {
			Context("in big endian", func() {
				It("writes the data length", func() {
					dataLen := 0x1337
					b := &bytes.Buffer{}
					f := &StreamFrame{
						StreamID:       1,
						Data:           bytes.Repeat([]byte{'f'}, dataLen),
						DataLenPresent: true,
						Offset:         0,
					}
					err := f.Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					minLength, _ := f.MinLength(0)
					Expect(b.Bytes()[0] & 0x20).To(Equal(uint8(0x20)))
					Expect(b.Bytes()[minLength-2 : minLength]).To(Equal([]byte{0x13, 0x37}))
				})
			})

			It("omits the data length field", func() {
				dataLen := 0x1337
				b := &bytes.Buffer{}
				f := &StreamFrame{
					StreamID:       1,
					Data:           bytes.Repeat([]byte{'f'}, dataLen),
					DataLenPresent: false,
					Offset:         0,
				}
				err := f.Write(b, protocol.VersionWhatever)
				Expect(err).ToNot(HaveOccurred())
				Expect(b.Bytes()[0] & 0x20).To(Equal(uint8(0)))
				Expect(b.Bytes()[1 : b.Len()-dataLen]).ToNot(ContainSubstring(string([]byte{0x37, 0x13})))
				minLength, _ := f.MinLength(0)
				f.DataLenPresent = true
				minLengthWithoutDataLen, _ := f.MinLength(0)
				Expect(minLength).To(Equal(minLengthWithoutDataLen - 2))
			})

			It("calculates the correcct min-length", func() {
				f := &StreamFrame{
					StreamID:       0xCAFE,
					Data:           []byte("foobar"),
					DataLenPresent: false,
					Offset:         0xDEADBEEF,
				}
				minLengthWithoutDataLen, _ := f.MinLength(0)
				f.DataLenPresent = true
				Expect(f.MinLength(0)).To(Equal(minLengthWithoutDataLen + 2))
			})
		})

		Context("offset lengths", func() {
			Context("in big endian", func() {
				It("does not write an offset if the offset is 0", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x0)))
				})

				It("writes a 2-byte offset if the offset is larger than 0", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0x1337,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x1 << 2)))
					Expect(b.Bytes()[2:4]).To(Equal([]byte{0x13, 0x37}))
				})

				It("writes a 3-byte offset", func() {
					b := &bytes.Buffer{}
					(&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0x13cafe,
					}).Write(b, versionBigEndian)
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x2 << 2)))
					Expect(b.Bytes()[2:5]).To(Equal([]byte{0x13, 0xca, 0xfe}))
				})

				It("writes a 4-byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0xdeadbeef,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x3 << 2)))
					Expect(b.Bytes()[2:6]).To(Equal([]byte{0xde, 0xad, 0xbe, 0xef}))
				})

				It("writes a 5-byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0x13deadbeef,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x4 << 2)))
					Expect(b.Bytes()[2:7]).To(Equal([]byte{0x13, 0xde, 0xad, 0xbe, 0xef}))
				})

				It("writes a 6-byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0xdeadbeefcafe,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x5 << 2)))
					Expect(b.Bytes()[2:8]).To(Equal([]byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}))
				})

				It("writes a 7-byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0x13deadbeefcafe,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x6 << 2)))
					Expect(b.Bytes()[2:9]).To(Equal([]byte{0x13, 0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}))
				})

				It("writes a 8-byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0x1337deadbeefcafe,
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x1c).To(Equal(uint8(0x7 << 2)))
					Expect(b.Bytes()[2:10]).To(Equal([]byte{0x13, 0x37, 0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}))
				})
			})

			Context("for the IETF format", func() {
				It("does not write an offset if the offset is 0", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0,
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x6).To(BeZero())
				})

				It("writes a 2 byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0x1337,
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x6).To(Equal(uint8(0x2)))
					Expect(b.Bytes()[2:4]).To(Equal([]byte{0x13, 0x37}))
				})

				It("writes a 4 byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0xdeadbeef,
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x6).To(Equal(uint8(0x4)))
					Expect(b.Bytes()[2:6]).To(Equal([]byte{0xde, 0xad, 0xbe, 0xef}))
				})

				It("writes a 8 byte offset", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 1,
						Data:     []byte("foobar"),
						Offset:   0xdeadbeefcafe,
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x6).To(Equal(uint8(0x6)))
					Expect(b.Bytes()[2:10]).To(Equal([]byte{0x0, 0x0, 0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}))
				})
			})
		})

		Context("lengths of StreamIDs", func() {
			Context("in big endian", func() {
				It("writes a 1 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 13,
						Data:     []byte("foobar"),
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x3).To(Equal(uint8(0x0)))
					Expect(b.Bytes()[1]).To(Equal(uint8(13)))
				})

				It("writes a 2 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 0xcafe,
						Data:     []byte("foobar"),
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x3).To(Equal(uint8(0x1)))
					Expect(b.Bytes()[1:3]).To(Equal([]byte{0xca, 0xfe}))
				})

				It("writes a 3 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 0x13beef,
						Data:     []byte("foobar"),
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x3).To(Equal(uint8(0x2)))
					Expect(b.Bytes()[1:4]).To(Equal([]byte{0x13, 0xbe, 0xef}))
				})

				It("writes a 4 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 0xdecafbad,
						Data:     []byte("foobar"),
					}).Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x3).To(Equal(uint8(0x3)))
					Expect(b.Bytes()[1:5]).To(Equal([]byte{0xde, 0xca, 0xfb, 0xad}))
				})

				It("writes a multiple byte StreamID, after the Stream length was already determined by MinLenght()", func() {
					b := &bytes.Buffer{}
					frame := &StreamFrame{
						StreamID: 0xdecafbad,
						Data:     []byte("foobar"),
					}
					frame.MinLength(0)
					err := frame.Write(b, versionBigEndian)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x3).To(Equal(uint8(0x3)))
					Expect(b.Bytes()[1:5]).To(Equal([]byte{0xde, 0xca, 0xfb, 0xad}))
				})
			})

			Context("for the IETF format", func() {
				It("writes a 1 byte Stream ID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 13,
						Data:     []byte("foobar"),
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x18).To(Equal(uint8(0x0)))
					Expect(b.Bytes()[1]).To(Equal(uint8(13)))
				})

				It("writes a 2 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 0xcafe,
						Data:     []byte("foobar"),
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x18).To(Equal(uint8(0x8)))
					Expect(b.Bytes()[1:3]).To(Equal([]byte{0xca, 0xfe}))
				})

				It("writes a 3 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 0x13beef,
						Data:     []byte("foobar"),
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x18).To(Equal(uint8(0x10)))
					Expect(b.Bytes()[1:4]).To(Equal([]byte{0x13, 0xbe, 0xef}))
				})

				It("writes a 4 byte StreamID", func() {
					b := &bytes.Buffer{}
					err := (&StreamFrame{
						StreamID: 0xdecafbad,
						Data:     []byte("foobar"),
					}).Write(b, versionIETFStreamFrame)
					Expect(err).ToNot(HaveOccurred())
					Expect(b.Bytes()[0] & 0x18).To(Equal(uint8(0x18)))
					Expect(b.Bytes()[1:5]).To(Equal([]byte{0xde, 0xca, 0xfb, 0xad}))
				})
			})
		})
	})

	Context("shortening of StreamIDs", func() {
		It("determines the length of a 1 byte StreamID", func() {
			f := &StreamFrame{StreamID: 0xFF}
			Expect(f.calculateStreamIDLength()).To(Equal(uint8(1)))
		})

		It("determines the length of a 2 byte StreamID", func() {
			f := &StreamFrame{StreamID: 0xFFFF}
			Expect(f.calculateStreamIDLength()).To(Equal(uint8(2)))
		})

		It("determines the length of a 3 byte StreamID", func() {
			f := &StreamFrame{StreamID: 0xFFFFFF}
			Expect(f.calculateStreamIDLength()).To(Equal(uint8(3)))
		})

		It("determines the length of a 4 byte StreamID", func() {
			f := &StreamFrame{StreamID: 0xFFFFFFFF}
			Expect(f.calculateStreamIDLength()).To(Equal(uint8(4)))
		})
	})

	Context("shortening of Offsets", func() {
		Context("for the gQUIC format", func() {
			It("determines length 0 of offset 0", func() {
				f := &StreamFrame{Offset: 0}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(0)))
			})

			It("determines the length of a 2 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(2)))
			})

			It("determines the length of a 2 byte offset, even if it would fit into 1 byte", func() {
				f := &StreamFrame{Offset: 0x1}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(2)))
			})

			It("determines the length of a 3 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(3)))
			})

			It("determines the length of a 4 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFFFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(4)))
			})

			It("determines the length of a 5 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFFFFFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(5)))
			})

			It("determines the length of a 6 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFFFFFFFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(6)))
			})

			It("determines the length of a 7 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFFFFFFFFFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(7)))
			})

			It("determines the length of an 8 byte offset", func() {
				f := &StreamFrame{Offset: 0xFFFFFFFFFFFFFFFF}
				Expect(f.getOffsetLength(versionGQUICStreamFrame)).To(Equal(protocol.ByteCount(8)))
			})
		})

		Context("for the IETF format", func() {
			It("determines length 0 of offset 0", func() {
				f := &StreamFrame{Offset: 0}
				Expect(f.getOffsetLength(versionIETFStreamFrame)).To(Equal(protocol.ByteCount(0)))
			})

			It("determines the length of a 2 byte offset", func() {
				f := &StreamFrame{Offset: 0x1}
				Expect(f.getOffsetLength(versionIETFStreamFrame)).To(Equal(protocol.ByteCount(2)))
			})

			It("determines the length of a 4 byte offset", func() {
				f := &StreamFrame{Offset: 0xffff + 1}
				Expect(f.getOffsetLength(versionIETFStreamFrame)).To(Equal(protocol.ByteCount(4)))
			})

			It("determines the length of a 8 byte offset", func() {
				f := &StreamFrame{Offset: 0xffffffff + 1}
				Expect(f.getOffsetLength(versionIETFStreamFrame)).To(Equal(protocol.ByteCount(8)))
			})
		})
	})

	Context("DataLen", func() {
		It("determines the length of the data", func() {
			frame := StreamFrame{
				Data: []byte("foobar"),
			}
			Expect(frame.DataLen()).To(Equal(protocol.ByteCount(6)))
		})
	})
})
