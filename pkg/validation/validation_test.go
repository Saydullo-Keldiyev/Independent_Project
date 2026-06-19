package validation

import (
	"html"
	"strings"
	"testing"
)

// ── Test DTOs ──────────────────────────────────────────────────────────────────

type TestBidDTO struct {
	AuctionID string  `validate:"required,uuid4"`
	Amount    float64 `validate:"required,bid_amount"`
	UserID    string  `validate:"required,uuid4"`
}

type TestUserDTO struct {
	Name  string `validate:"required,max_text=50"`
	Email string `validate:"required,email"`
	Bio   string `validate:"max_text"`
}

type TestCreateAuctionDTO struct {
	Title       string  `validate:"required,max_text=200"`
	Description string  `validate:"required,max_text"`
	StartPrice  float64 `validate:"required,gt=0"`
}

// ── ValidateStruct Tests ───────────────────────────────────────────────────────

func TestValidateStruct_ValidBidDTO(t *testing.T) {
	v := New()
	dto := TestBidDTO{
		AuctionID: "550e8400-e29b-41d4-a716-446655440000",
		Amount:    100.50,
		UserID:    "6ba7b810-9dad-41d4-80b4-00c04fd430c8",
	}

	errs := v.ValidateStruct(dto)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateStruct_InvalidBidDTO(t *testing.T) {
	v := New()
	dto := TestBidDTO{
		AuctionID: "not-a-uuid",
		Amount:    -5.0,
		UserID:    "",
	}

	errs := v.ValidateStruct(dto)
	if len(errs) == 0 {
		t.Error("expected validation errors, got none")
	}

	// Should have errors for AuctionID (uuid4), Amount (bid_amount), UserID (required)
	fieldNames := make(map[string]bool)
	for _, e := range errs {
		fieldNames[e.Field] = true
	}

	if !fieldNames["auctionID"] {
		t.Error("expected error for auctionID field")
	}
	if !fieldNames["amount"] {
		t.Error("expected error for amount field")
	}
	if !fieldNames["userID"] {
		t.Error("expected error for userID field")
	}
}

func TestValidateStruct_MaxTextDefault(t *testing.T) {
	v := New()
	dto := TestUserDTO{
		Name:  "John",
		Email: "john@example.com",
		Bio:   strings.Repeat("a", 1001), // exceeds default 1000 limit
	}

	errs := v.ValidateStruct(dto)
	if len(errs) == 0 {
		t.Error("expected validation error for bio exceeding 1000 chars")
	}

	found := false
	for _, e := range errs {
		if e.Field == "bio" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for bio field, got errors for other fields")
	}
}

func TestValidateStruct_MaxTextCustomLimit(t *testing.T) {
	v := New()
	dto := TestUserDTO{
		Name:  strings.Repeat("a", 51), // exceeds custom 50 limit
		Email: "john@example.com",
		Bio:   "short bio",
	}

	errs := v.ValidateStruct(dto)
	if len(errs) == 0 {
		t.Error("expected validation error for name exceeding 50 chars")
	}
}

// ── SanitizeString Tests ───────────────────────────────────────────────────────

func TestSanitizeString_HTMLSpecialChars(t *testing.T) {
	v := New()

	tests := []struct {
		input    string
		expected string
	}{
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"a & b", "a &amp; b"},
		{`"hello"`, "&#34;hello&#34;"},
		{"no special chars", "no special chars"},
		{"", ""},
		{"<div class=\"test\">It's a 'test' & more</div>",
			"&lt;div class=&#34;test&#34;&gt;It&#39;s a &#39;test&#39; &amp; more&lt;/div&gt;"},
	}

	for _, tt := range tests {
		result := v.SanitizeString(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeString_RoundTrip(t *testing.T) {
	v := New()

	inputs := []string{
		"hello world",
		"<script>alert('xss')</script>",
		"a & b < c > d \"e\" f'g",
		"normal text without special chars",
		"",
		"mixed <b>bold</b> & 'quoted' \"stuff\"",
	}

	for _, input := range inputs {
		sanitized := v.SanitizeString(input)

		// Verify no unescaped HTML special characters exist
		if strings.ContainsAny(sanitized, "<>") && !strings.Contains(sanitized, "&lt;") && !strings.Contains(sanitized, "&gt;") {
			t.Errorf("SanitizeString(%q) contains unescaped < or >", input)
		}

		// Round-trip: unescape should give back original
		// html.UnescapeString handles &amp; &lt; &gt; &quot;
		// We also need to handle &#39; -> '
		unescaped := html.UnescapeString(sanitized)
		// html.UnescapeString also handles &#39;
		if unescaped != input {
			t.Errorf("Round-trip failed: input=%q, sanitized=%q, unescaped=%q", input, sanitized, unescaped)
		}
	}
}

// ── ValidateUUID Tests ─────────────────────────────────────────────────────────

func TestValidateUUID_Valid(t *testing.T) {
	v := New()

	validUUIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-41d4-80b4-00c04fd430c8",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"7c9e6679-7425-40de-944b-e07fc1f90ae7",
	}

	for _, uuid := range validUUIDs {
		if !v.ValidateUUID(uuid) {
			t.Errorf("expected %q to be a valid UUID v4", uuid)
		}
	}
}

func TestValidateUUID_Invalid(t *testing.T) {
	v := New()

	invalidUUIDs := []string{
		"",
		"not-a-uuid",
		"550e8400-e29b-11d4-a716-446655440000", // version 1, not 4
		"550e8400-e29b-41d4-c716-446655440000", // invalid variant (c)
		"550e8400e29b41d4a716446655440000",     // no dashes
		"550e8400-e29b-41d4-a716-44665544000",  // too short
		"550e8400-e29b-41d4-a716-4466554400000", // too long
		"ZZZZZZZZ-ZZZZ-4ZZZ-8ZZZ-ZZZZZZZZZZZZ", // non-hex
	}

	for _, uuid := range invalidUUIDs {
		if v.ValidateUUID(uuid) {
			t.Errorf("expected %q to be an invalid UUID v4", uuid)
		}
	}
}

// ── ValidatePayloadSize Tests ──────────────────────────────────────────────────

func TestValidatePayloadSize(t *testing.T) {
	v := New()

	tests := []struct {
		size     int64
		expected bool
	}{
		{0, true},
		{1024, true},
		{MaxPayloadSize, true},
		{MaxPayloadSize + 1, false},
		{2 * MaxPayloadSize, false},
	}

	for _, tt := range tests {
		result := v.ValidatePayloadSize(tt.size)
		if result != tt.expected {
			t.Errorf("ValidatePayloadSize(%d) = %v, want %v", tt.size, result, tt.expected)
		}
	}
}

// ── ValidateTextLength Tests ───────────────────────────────────────────────────

func TestValidateTextLength(t *testing.T) {
	v := New()

	tests := []struct {
		text      string
		maxLength int
		expected  bool
	}{
		{"short", 0, true},                               // default 1000
		{strings.Repeat("a", 1000), 0, true},             // exactly 1000 with default
		{strings.Repeat("a", 1001), 0, false},            // exceeds default
		{strings.Repeat("a", 50), 50, true},              // custom limit exact
		{strings.Repeat("a", 51), 50, false},             // exceeds custom
		{"", 0, true},                                    // empty string
	}

	for _, tt := range tests {
		result := v.ValidateTextLength(tt.text, tt.maxLength)
		if result != tt.expected {
			t.Errorf("ValidateTextLength(len=%d, max=%d) = %v, want %v",
				len(tt.text), tt.maxLength, result, tt.expected)
		}
	}
}

// ── ValidateBidAmount Tests ────────────────────────────────────────────────────

func TestValidateBidAmount_Valid(t *testing.T) {
	v := New()

	validAmounts := []float64{
		0.01,
		1.00,
		100.50,
		999_999_999.99,
		0.99,
		500.00,
		123456.78,
	}

	for _, amount := range validAmounts {
		errs := v.ValidateBidAmount(amount)
		if len(errs) != 0 {
			t.Errorf("expected no errors for amount %f, got %v", amount, errs)
		}
	}
}

func TestValidateBidAmount_Invalid(t *testing.T) {
	v := New()

	tests := []struct {
		amount      float64
		expectedMsg string
	}{
		{0, "must be positive"},
		{-1.0, "must be positive"},
		{-100.50, "must be positive"},
		{1_000_000_000.00, "must not exceed"},
		{999_999_999.999, "must have at most 2 decimal places"},
		{10.123, "must have at most 2 decimal places"},
		{1.001, "must have at most 2 decimal places"},
	}

	for _, tt := range tests {
		errs := v.ValidateBidAmount(tt.amount)
		if len(errs) == 0 {
			t.Errorf("expected errors for amount %f, got none", tt.amount)
			continue
		}

		found := false
		for _, e := range errs {
			if strings.Contains(e.Message, tt.expectedMsg) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected error containing %q for amount %f, got %v",
				tt.expectedMsg, tt.amount, errs)
		}
	}
}

// ── ValidateQueryParam Tests ───────────────────────────────────────────────────

func TestValidateQueryParam_UUID(t *testing.T) {
	v := New()

	// Valid UUID
	err := v.ValidateQueryParam("id", "550e8400-e29b-41d4-a716-446655440000", "uuid4")
	if err != nil {
		t.Errorf("expected no error for valid UUID, got %v", err)
	}

	// Invalid UUID
	err = v.ValidateQueryParam("id", "not-a-uuid", "uuid4")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
	if err != nil && err.Field != "id" {
		t.Errorf("expected field 'id', got %q", err.Field)
	}
}

func TestValidateQueryParam_Required(t *testing.T) {
	v := New()

	err := v.ValidateQueryParam("name", "value", "required")
	if err != nil {
		t.Errorf("expected no error for non-empty value, got %v", err)
	}

	err = v.ValidateQueryParam("name", "", "required")
	if err == nil {
		t.Error("expected error for empty required parameter")
	}

	err = v.ValidateQueryParam("name", "   ", "required")
	if err == nil {
		t.Error("expected error for whitespace-only required parameter")
	}
}

func TestValidateQueryParam_Numeric(t *testing.T) {
	v := New()

	validNums := []string{"123", "0", "3.14", "-1", "999.99"}
	for _, num := range validNums {
		err := v.ValidateQueryParam("price", num, "numeric")
		if err != nil {
			t.Errorf("expected no error for numeric %q, got %v", num, err)
		}
	}

	invalidNums := []string{"abc", "12a", "1.2.3", "$100"}
	for _, num := range invalidNums {
		err := v.ValidateQueryParam("price", num, "numeric")
		if err == nil {
			t.Errorf("expected error for non-numeric %q", num)
		}
	}
}

func TestValidateQueryParam_MaxText(t *testing.T) {
	v := New()

	err := v.ValidateQueryParam("search", strings.Repeat("a", 1000), "max_text")
	if err != nil {
		t.Errorf("expected no error for 1000 char string, got %v", err)
	}

	err = v.ValidateQueryParam("search", strings.Repeat("a", 1001), "max_text")
	if err == nil {
		t.Error("expected error for 1001 char string")
	}
}

// ── FieldError Tests ───────────────────────────────────────────────────────────

func TestFieldError_Error(t *testing.T) {
	fe := FieldError{
		Field:   "amount",
		Message: "must be positive",
		Value:   -5.0,
	}

	expected := "field 'amount': must be positive"
	if fe.Error() != expected {
		t.Errorf("FieldError.Error() = %q, want %q", fe.Error(), expected)
	}
}
