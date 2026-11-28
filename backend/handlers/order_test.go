package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/saveplate/backend/models"
	"github.com/saveplate/backend/services"
)

// TestCreateOrderRequest_Validation menguji validasi request sesuai handler logic
func TestCreateOrderRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     services.CreateOrderRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid request",
			req: services.CreateOrderRequest{
				ProductID:    1,
				Quantity:     2,
				BillingName:  "John Doe",
				BillingEmail: "john@example.com",
				BillingPhone: "08123456789",
			},
			wantErr: false,
		},
		{
			name: "ProductID = 0 should fail validation",
			req: services.CreateOrderRequest{
				ProductID:    0,
				Quantity:     2,
				BillingName:  "John Doe",
				BillingEmail: "john@example.com",
				BillingPhone: "08123456789",
			},
			wantErr: true,
			errMsg:  "Product ID wajib diisi",
		},
		{
			name: "Quantity = 0 should fail validation",
			req: services.CreateOrderRequest{
				ProductID:    1,
				Quantity:     0,
				BillingName:  "John Doe",
				BillingEmail: "john@example.com",
				BillingPhone: "08123456789",
			},
			wantErr: true,
			errMsg:  "Jumlah harus minimal 1",
		},
		{
			name: "Quantity negative should fail validation",
			req: services.CreateOrderRequest{
				ProductID:    1,
				Quantity:     -1,
				BillingName:  "John Doe",
				BillingEmail: "john@example.com",
				BillingPhone: "08123456789",
			},
			wantErr: true,
			errMsg:  "Jumlah harus minimal 1",
		},
		{
			name: "Empty billing_name should fail validation",
			req: services.CreateOrderRequest{
				ProductID:    1,
				Quantity:     2,
				BillingName:  "",
				BillingEmail: "john@example.com",
				BillingPhone: "08123456789",
			},
			wantErr: true,
			errMsg:  "Nama pemesan wajib diisi",
		},
		{
			name: "Empty billing_phone should fail validation",
			req: services.CreateOrderRequest{
				ProductID:    1,
				Quantity:     2,
				BillingName:  "John Doe",
				BillingEmail: "john@example.com",
				BillingPhone: "",
			},
			wantErr: true,
			errMsg:  "Nomor telepon wajib diisi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulasi validasi manual seperti di handler CreateOrder
			var errMsg string
			if tt.req.ProductID == 0 {
				errMsg = "Product ID wajib diisi"
			} else if tt.req.Quantity < 1 {
				errMsg = "Jumlah harus minimal 1"
			} else if tt.req.BillingName == "" {
				errMsg = "Nama pemesan wajib diisi"
			} else if tt.req.BillingPhone == "" {
				errMsg = "Nomor telepon wajib diisi"
			}

			hasError := errMsg != ""
			if hasError != tt.wantErr {
				t.Errorf("validation error = %v, wantErr %v", hasError, tt.wantErr)
			}

			if tt.wantErr && errMsg != tt.errMsg {
				t.Errorf("error message = %q, want %q", errMsg, tt.errMsg)
			}
		})
	}
}

// TestCreateOrderRequest_JSONParsing menguji parsing JSON ke struct
func TestCreateOrderRequest_JSONParsing(t *testing.T) {
	tests := []struct {
		name      string
		jsonInput string
		wantErr   bool
		errType   string
	}{
		{
			name: "Valid JSON with correct types",
			jsonInput: `{
				"product_id": 1,
				"quantity": 2,
				"billing_name": "John Doe",
				"billing_email": "john@example.com",
				"billing_phone": "08123456789"
			}`,
			wantErr: false,
		},
		{
			name: "product_id as string should fail unmarshal",
			jsonInput: `{
				"product_id": "1",
				"quantity": 2,
				"billing_name": "John Doe",
				"billing_email": "john@example.com",
				"billing_phone": "08123456789"
			}`,
			wantErr: true,
			errType: "type_mismatch",
		},
		{
			name: "quantity as string should fail unmarshal",
			jsonInput: `{
				"product_id": 1,
				"quantity": "2",
				"billing_name": "John Doe",
				"billing_email": "john@example.com",
				"billing_phone": "08123456789"
			}`,
			wantErr: true,
			errType: "type_mismatch",
		},
		{
			name:      "Invalid JSON syntax",
			jsonInput: `{"product_id": 1, "quantity": 2`,
			wantErr:   true,
			errType:   "syntax",
		},
		{
			name: "Valid JSON with optional fields missing",
			jsonInput: `{
				"product_id": 1,
				"quantity": 2,
				"billing_name": "John Doe",
				"billing_phone": "08123456789"
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req services.CreateOrderRequest
			err := json.Unmarshal([]byte(tt.jsonInput), &req)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verifikasi tipe data setelah parsing berhasil
			if !tt.wantErr {
				if req.ProductID == 0 && tt.jsonInput != "" {
					t.Error("ProductID should not be 0 after successful unmarshal")
				}
				if req.Quantity == 0 && tt.jsonInput != "" {
					t.Error("Quantity should not be 0 after successful unmarshal")
				}
			}
		})
	}
}

func TestUpdateOrderStatus_BlocksCompletedBypass(t *testing.T) {
	app := fiber.New()
	h := NewOrderHandler(nil)
	app.Patch("/orders/:id/status", func(c *fiber.Ctx) error {
		c.Locals("user", &JWTClaims{UserID: 7, Role: models.RoleUmkm})
		return h.UpdateOrderStatus(c)
	})

	req := httptest.NewRequest("PATCH", "/orders/1/status", bytes.NewBufferString(`{"status":"COMPLETED"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400 when attempting COMPLETED bypass", resp.StatusCode)
	}
}

func TestValidatePickupEligibility(t *testing.T) {
	now := time.Now()
	future := now.Add(2 * time.Hour)
	past := now.Add(-1 * time.Minute)
	code := "SAVE1234"

	base := models.Order{
		Status:         models.OrderReadyForPickup,
		PaymentStatus:  models.PaymentPaid,
		PickupCode:     &code,
		PickupDeadline: &future,
	}

	if err := validatePickupEligibility(base, "  save1234 ", now); err != nil {
		t.Fatalf("expected valid case-insensitive trimmed code to pass, got: %v", err)
	}

	// Wrong code
	if err := validatePickupEligibility(base, "WRONG999", now); err == nil {
		t.Fatal("expected wrong pickup code to fail")
	}

	// Expired deadline (P2 enforcement)
	expired := base
	expired.PickupDeadline = &past
	if err := validatePickupEligibility(expired, code, now); err == nil {
		t.Fatal("expected pickup past deadline to fail")
	}

	// Unpaid order
	unpaid := base
	unpaid.PaymentStatus = models.PaymentPending
	if err := validatePickupEligibility(unpaid, code, now); err == nil {
		t.Fatal("expected unpaid order to fail")
	}

	// Second verification attempt (already COMPLETED — P3 idempotence)
	alreadyDone := base
	alreadyDone.Status = models.OrderCompleted
	if err := validatePickupEligibility(alreadyDone, code, now); err == nil {
		t.Fatal("expected second pickup verification on COMPLETED order to fail")
	}
}

func TestOrderOwnershipAndPickupCodeMasking(t *testing.T) {
	code := "SECRET99"
	order := models.Order{
		ID:         1,
		CustomerID: 10,
		PickupCode: &code,
		Product:    models.Product{ID: 5, UmkmID: 20},
	}

	ownerUMKM := &JWTClaims{UserID: 20, Role: models.RoleUmkm}
	otherUMKM := &JWTClaims{UserID: 99, Role: models.RoleUmkm}
	ownerCust := &JWTClaims{UserID: 10, Role: models.RoleCustomer}
	otherCust := &JWTClaims{UserID: 11, Role: models.RoleCustomer}
	admin := &JWTClaims{UserID: 1, Role: models.RoleAdmin}

	if !canAccessOrder(ownerUMKM, order, false) || canAccessOrder(otherUMKM, order, false) {
		t.Fatal("UMKM ownership check failed")
	}
	if !canAccessOrder(ownerCust, order, true) || canAccessOrder(ownerCust, order, false) || canAccessOrder(otherCust, order, true) {
		t.Fatal("Customer ownership check failed")
	}
	if !canAccessOrder(admin, order, false) {
		t.Fatal("Admin access check failed")
	}

	// Masking: UMKM tidak boleh menerima pickup_code; customer tetap menerima
	custCopy := order
	maskOrderForRole(models.RoleCustomer, &custCopy)
	if custCopy.PickupCode == nil || *custCopy.PickupCode != code {
		t.Fatal("customer must retain pickup_code")
	}
	umkmCopy := order
	maskOrderForRole(models.RoleUmkm, &umkmCopy)
	if umkmCopy.PickupCode != nil {
		t.Fatal("UMKM must not receive pickup_code")
	}
}

func TestProductWeightValidationAndOwnership(t *testing.T) {
	if err := validateProductInput("Nasi Kotak", 5, 350); err != nil {
		t.Fatalf("expected valid product input, got: %v", err)
	}
	if err := validateProductInput("Nasi Kotak", 5, 0); err == nil {
		t.Fatal("expected weight=0 to fail")
	}
	if err := validateProductInput("Nasi Kotak", 5, -100); err == nil {
		t.Fatal("expected negative weight to fail")
	}
	if err := validateProductInput("", 5, 300); err == nil {
		t.Fatal("expected empty name to fail")
	}
	if !canManageProduct(&JWTClaims{UserID: 5, Role: models.RoleUmkm}, 5) {
		t.Fatal("owner UMKM should manage product")
	}
	if canManageProduct(&JWTClaims{UserID: 6, Role: models.RoleUmkm}, 5) {
		t.Fatal("other UMKM must not manage product")
	}
	if !canManageProduct(&JWTClaims{UserID: 1, Role: models.RoleAdmin}, 5) {
		t.Fatal("admin should manage product")
	}
}
