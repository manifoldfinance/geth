package rawdb

import (
  "encoding/json"
  "fmt"
  "bytes"
  "strings"
  "strconv"
  lru "github.com/hashicorp/golang-lru"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  s3 "github.com/aws/aws-sdk-go/service/s3"
	// "github.com/aws/aws-sdk-go/service/s3/s3manager"
  "github.com/golang/snappy"
  "github.com/ethereum/go-ethereum/ethdb"
  "io/ioutil"
)


type s3freezer struct {
  sess *session.Session
  cache *lru.Cache
  bucket string
  root string
  count uint64
}

type s3record struct {
  Hash []byte `json:"x"`
  Header []byte `json:"h"`
  Body []byte `json:"b"`
  Receipt []byte `json:"r"`
  Td []byte `json:"d"`
}

func numToPath(number uint64) string {
  h := fmt.Sprintf("%0.7x", number)
  x := len(h)
  return fmt.Sprintf("%v/%v/%v", h[:x-4], h[x-4:x-2], h[x-2:])
}

func NewS3Freezer(path string, cacheSize int) (ethdb.AncientStore, error) {
  path = strings.TrimPrefix(path, "s3://")
  parts := strings.SplitN(path, "/", 2)
  cache, err := lru.New(cacheSize)
  if err != nil { return nil, err }
  freezer := &s3freezer{
    cache: cache,
    sess: session.Must(session.NewSession()),
    bucket: parts[0],
    root: parts[1],
  }
  svc := s3.New(freezer.sess)
  output, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
    Bucket: &freezer.bucket,
    Delimiter: aws.String("/"),
    Prefix: &freezer.root,
  })
  for output.NextContinuationToken != nil {
    if err != nil { return nil, err }
    output, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
      Bucket: &freezer.bucket,
      Delimiter: aws.String("/"),
      Prefix: &freezer.root,
      ContinuationToken: output.NextContinuationToken,
    })
  }
  l1prefix := output.CommonPrefixes[len(output.CommonPrefixes) - 1].Prefix
  output, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
    Bucket: &freezer.bucket,
    Delimiter: aws.String("/"),
    Prefix: l1prefix,
  })
  l2prefix := output.CommonPrefixes[len(output.CommonPrefixes)-1].Prefix
  output, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
    Bucket: &freezer.bucket,
    Delimiter: aws.String("/"),
    Prefix: l2prefix,
  })
  highest := output.Contents[len(output.Contents)].Key
  count, err := strconv.ParseInt(strings.Join(strings.Split(strings.TrimPrefix(*highest, freezer.root), "/"), ""), 16, 64)
  if err != nil { return nil, err }
  freezer.count = uint64(count)
  return freezer, nil
}

// HasAncient returns an indicator whether the specified data exists in the
// ancient store.
func (f *s3freezer) HasAncient(kind string, number uint64) (bool, error) {
  key := numToPath(number)
  svc := s3.New(f.sess)
  if _, ok := f.cache.Get(key); ok { return true, nil }
  _, err := svc.HeadObject(&s3.HeadObjectInput{
    Bucket: &f.bucket,
    Key: aws.String(fmt.Sprintf("%v/%v", f.root, key)),
  })
  return err == nil, nil
}

// Ancient retrieves an ancient binary blob from the append-only immutable files.
func (f *s3freezer) Ancient(kind string, number uint64) ([]byte, error) {
  key := numToPath(number)
  var content []byte
  svc := s3.New(f.sess)
  if content, ok := f.cache.Get(key); !ok {
    value, err := svc.GetObject(&s3.GetObjectInput{
      Bucket: &f.bucket,
      Key: aws.String(fmt.Sprintf("%v/%v", f.root, key)),
    })
    if err != nil {
      return nil, err
    }
    content, err = ioutil.ReadAll(snappy.NewReader(value.Body))
    if err != nil {
      return nil, err
    }
    f.cache.Add(key, content)
  }
  record := &s3record{}
  err := json.Unmarshal(content, &record)
  switch kind {
  case freezerHeaderTable:
    return record.Header, err
  case freezerHashTable:
    return record.Hash, err
  case freezerBodiesTable:
    return record.Body, err
  case freezerReceiptTable:
    return record.Receipt, err
  case freezerDifficultyTable:
    return record.Td, err
  default:
    return nil, fmt.Errorf("Unknown kind '%v'", kind)
  }
}

// Ancients returns the ancient item numbers in the ancient store.
func (f *s3freezer) Ancients() (uint64, error) {
  return f.count, nil
}

// AncientSize returns the ancient size of the specified category.
func (f *s3freezer) AncientSize(kind string) (uint64, error) {
  return f.count, nil
}
// AppendAncient injects all binary blobs belong to block at the end of the
// append-only immutable table files.
func (f *s3freezer) AppendAncient(number uint64, hash, header, body, receipt, td []byte) error {
  record := s3record{
    Hash: hash,
    Header: header,
    Body: body,
    Receipt: receipt,
    Td: td,
  }
  data, err := json.Marshal(record)
  buff := &bytes.Buffer{}
  writer := snappy.NewWriter(buff)
  writer.Write(data)
  writer.Flush()
  svc := s3.New(f.sess)
  key := numToPath(number)
  _, err = svc.PutObject(&s3.PutObjectInput{
    Bucket: &f.bucket,
    Key: aws.String(fmt.Sprintf("%v/%v", f.root, key)),
    Body: bytes.NewReader(buff.Bytes()),
  })
  if err == nil { f.count++ }
  return err
}

// TruncateAncients discards all but the first n ancient data from the ancient store.
func (f *s3freezer) TruncateAncients(n uint64) error {
  if n < f.count {
    f.count = n
  }
  return nil
}

// Sync is a no-op, as we will write content as AppendAncient is called
func (f *s3freezer) Sync() error {
  return nil
}

func (f *s3freezer) Close() error {
  return nil
}
