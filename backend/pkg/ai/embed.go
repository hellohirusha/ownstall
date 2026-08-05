package ai

import (
	"context"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// EmbeddingDim is the width of the vectors this package produces. It
// must match the vector(N) column in migration 008 — changing it needs
// a migration and a full re-index.
const EmbeddingDim = 256

// LexicalHashModel labels vectors produced by LexicalEmbedder, stored
// in product_embeddings.model so a later switch to a hosted embedding
// model can find and replace the rows it supersedes.
const LexicalHashModel = "lexical-hash-v1"

// Document is the text of one product, split by how much each part
// says about what the product actually is.
type Document struct {
	Title string   // product name — the strongest signal
	Tags  []string // curated by the creator, so also high signal
	Body  string   // description — longest, noisiest
}

// Field weights. A name match should outrank a description match by a
// wide margin: two products both described as "waterproof" are far
// less alike than two both named "holographic sticker sheet".
const (
	titleWeight = 3.0
	tagWeight   = 2.0
	bodyWeight  = 1.0
)

// Embedder turns a product into a vector. The interface exists so a
// hosted embedding model (Voyage, OpenAI) can be dropped in as a
// second implementation without touching the recommendation service.
type Embedder interface {
	Embed(ctx context.Context, doc Document) ([]float32, error)
	Model() string
	Dim() int
}

// LexicalEmbedder builds vectors locally with weighted feature
// hashing. No API key, no network call, no rate limit, no cost, and
// identical output for identical input — which also makes the
// evaluation script reproducible offline.
//
// This measures lexical overlap, not meaning: it will not learn that
// "tee" and "t-shirt" are the same thing. For a catalogue where names
// and tags are written by the same creator, term overlap is a solid
// similarity signal, and it degrades honestly rather than
// hallucinating relationships.
type LexicalEmbedder struct{}

func (LexicalEmbedder) Model() string { return LexicalHashModel }
func (LexicalEmbedder) Dim() int      { return EmbeddingDim }

// Embed never returns an error; the signature matches Embedder so
// network-backed implementations can report failures.
func (LexicalEmbedder) Embed(_ context.Context, doc Document) ([]float32, error) {
	counts := make(map[string]float64)

	addField(counts, doc.Title, titleWeight)
	for _, tag := range doc.Tags {
		addField(counts, tag, tagWeight)
	}
	addField(counts, doc.Body, bodyWeight)

	vec := make([]float32, EmbeddingDim)
	for term, weight := range counts {
		// Sublinear term frequency: the tenth "sticker" in a
		// description says much less than the first.
		value := 1 + math.Log(weight)

		bucket, sign := hashTerm(term)
		vec[bucket] += float32(sign * value)
	}

	normalize(vec)
	return vec, nil
}

// addField tokenizes one field and accumulates unigrams and bigrams at
// the field's weight.
func addField(counts map[string]float64, text string, weight float64) {
	tokens := tokenize(text)
	for i, tok := range tokens {
		counts[tok] += weight
		// Bigrams keep multi-word identity: "sticker sheet" is its own
		// term, not the sum of two generic ones.
		if i+1 < len(tokens) {
			counts[tok+"_"+tokens[i+1]] += weight
		}
	}
}

// tokenize lowercases and splits on anything that is not a letter or
// digit, so alphanumeric product terms ("3x3", "a4") survive intact.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 || stopwords[f] {
			continue
		}
		// Bare quantities ("12", "2024") match across unrelated
		// products and add noise.
		if _, err := strconv.Atoi(f); err == nil {
			continue
		}
		tokens = append(tokens, f)
	}
	return tokens
}

// hashTerm maps a term to a bucket and a sign. The sign comes from a
// high bit that the modulo does not consume, so collisions between
// unrelated terms cancel out on average instead of always inflating
// the same bucket.
func hashTerm(term string) (int, float64) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(term)) // hash.Hash never returns an error
	sum := h.Sum64()

	bucket := int(sum % uint64(EmbeddingDim))
	if sum&(1<<63) != 0 {
		return bucket, -1
	}
	return bucket, 1
}

// normalize scales the vector to unit length so cosine distance
// reduces to a dot product and long descriptions do not outrank short
// ones purely on length.
func normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		return
	}

	inv := float32(1 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= inv
	}
}

// VectorLiteral renders a vector in pgvector's text input format,
// which lets the driver send it as a plain string cast with $n::vector
// — no pgvector Go dependency required.
func VectorLiteral(vec []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// stopwords are terms too common in product copy to carry similarity
// signal. Deliberately short: over-filtering costs more than it saves
// once vectors are length-normalized.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "this": true,
	"that": true, "you": true, "your": true, "our": true, "are": true,
	"from": true, "all": true, "can": true, "has": true, "have": true,
	"will": true, "its": true, "it": true, "is": true, "of": true,
	"in": true, "on": true, "to": true, "at": true, "by": true,
	"or": true, "an": true, "as": true, "be": true, "we": true,
	"perfect": true, "great": true, "best": true, "quality": true,
	"product": true, "item": true, "made": true, "get": true,
}
