package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

type api struct {
	db                   *pgxpool.Pool
	redis                *redis.Client
	jwtSecret            []byte
	whatsappToken        string
	whatsappPhoneID      string
	whatsappTemplate     string
	whatsappGraphVersion string
	otpDevMode           bool
	integrationService   *posintegration.Service
	integrationKey       []byte
}
type envelope struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *apiError `json:"error,omitempty"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type customer struct {
	ID             string     `json:"id"`
	FirstName      string     `json:"firstName"`
	LastName       string     `json:"lastName"`
	Phone          string     `json:"phone"`
	Email          string     `json:"email"`
	Birthday       *time.Time `json:"birthday,omitempty"`
	TotalPoints    int        `json:"totalPoints"`
	TotalVisits    int        `json:"totalVisits"`
	Level          string     `json:"level"`
	CreatedAt      time.Time  `json:"createdAt"`
	FavoriteBranch string     `json:"favoriteBranch,omitempty"`
	LastBranch     string     `json:"lastBranch,omitempty"`
}
type customerInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Birthday  string `json:"birthday"`
}
type visitInput struct {
	CustomerID string `json:"customerId"`
	BranchID   string `json:"branchId"`
	Comment    string `json:"comment"`
}

func New(db *pgxpool.Pool, redisClient *redis.Client, jwtSecret string) http.Handler {
	a := &api{db: db, redis: redisClient, jwtSecret: []byte(jwtSecret), whatsappToken: os.Getenv("WHATSAPP_ACCESS_TOKEN"), whatsappPhoneID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), whatsappTemplate: envOr("WHATSAPP_OTP_TEMPLATE", "tappix_login_code"), whatsappGraphVersion: envOr("WHATSAPP_GRAPH_VERSION", "v23.0"), otpDevMode: os.Getenv("OTP_DEV_MODE") == "true", integrationService: posintegration.NewService(db), integrationKey: integrationEncryptionKey(jwtSecret)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", a.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", a.forgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", a.resetPassword)
	mux.HandleFunc("POST /api/v1/customer/register", a.customerRegister)
	mux.HandleFunc("POST /api/v1/customer/login", a.customerLogin)
	mux.HandleFunc("POST /api/v1/customer/otp/request", a.customerOTPRequest)
	mux.HandleFunc("POST /api/v1/customer/otp/verify", a.customerOTPVerify)
	mux.HandleFunc("GET /api/v1/public/guest/{token}", a.publicGuestPortal)
	mux.HandleFunc("GET /api/v1/public/referral/{code}", a.publicReferral)
	mux.HandleFunc("GET /api/v1/public/sites/{slug}", a.publicWebsite)
	mux.HandleFunc("POST /api/v1/public/sites/{slug}/bookings", a.publicCreateBooking)
	mux.HandleFunc("GET /api/v1/public/files/{id}", a.publicFile)
	mux.HandleFunc("GET /api/v1/public/reports/{id}", a.publicReportArtifact)
	mux.HandleFunc("POST /api/v1/integrations/inbound/{key}", a.integrationInboundWebhook)
	mux.HandleFunc("POST /api/v1/integrations/poster/{key}", a.posterWebhook)
	mux.Handle("POST /api/v1/integrations/transactions/quote", a.authenticateAPIKey("transactions.read", http.HandlerFunc(a.canonicalTransactionQuote)))
	mux.Handle("POST /api/v1/integrations/transactions", a.authenticateAPIKey("transactions.write", http.HandlerFunc(a.canonicalTransactionCreate)))
	mux.Handle("GET /api/v1/integrations/transactions/{id}", a.authenticateAPIKey("transactions.read", http.HandlerFunc(a.canonicalTransactionGet)))
	mux.Handle("POST /api/v1/integrations/transactions/{id}/refund", a.authenticateAPIKey("transactions.refund", http.HandlerFunc(a.canonicalTransactionRefund)))
	mux.Handle("POST /api/v1/integration-jobs/{id}/retry", a.authenticateAPIKey("jobs.retry", http.HandlerFunc(a.integrationJobRetry)))
	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", a.me)
	protected.HandleFunc("GET /api/v1/auth/sessions", a.listSessions)
	protected.HandleFunc("DELETE /api/v1/auth/sessions/{id}", a.deleteSession)
	protected.Handle("POST /api/v1/auth/mfa/setup", a.requireRoles(http.HandlerFunc(a.mfaSetup), "super_admin", "company_owner"))
	protected.Handle("POST /api/v1/auth/mfa/enable", a.requireRoles(http.HandlerFunc(a.mfaEnable), "super_admin", "company_owner"))
	protected.Handle("POST /api/v1/auth/mfa/disable", a.requireRoles(http.HandlerFunc(a.mfaDisable), "super_admin", "company_owner"))
	protected.Handle("GET /api/v1/admin/support-sessions", a.requireRoles(http.HandlerFunc(a.listSupportSessions), "super_admin"))
	protected.Handle("POST /api/v1/admin/support-sessions", a.requireRoles(http.HandlerFunc(a.startSupportSession), "super_admin"))
	protected.Handle("DELETE /api/v1/admin/support-sessions/{id}", a.requireRoles(http.HandlerFunc(a.revokeSupportSession), "super_admin"))
	protected.Handle("GET /api/v1/workspaces", a.requireRoles(http.HandlerFunc(a.listWorkspaces), "company_owner", "employee"))
	protected.Handle("POST /api/v1/workspaces/{id}/switch", a.requireRoles(http.HandlerFunc(a.switchWorkspace), "company_owner", "employee"))
	protected.Handle("GET /api/v1/dashboard", a.requirePermission("workspace.read", http.HandlerFunc(a.dashboard)))
	protected.Handle("GET /api/v1/customers", a.requireModule("crm", a.requirePermission("customers.read", http.HandlerFunc(a.listCustomers))))
	protected.Handle("POST /api/v1/staff/customers/lookup", a.requireModule("loyalty", a.requirePermission("customers.read", http.HandlerFunc(a.customerByCode))))
	protected.Handle("GET /api/v1/customers/export", a.requireModule("crm", a.requireRoles(http.HandlerFunc(a.exportCustomers), "company_owner")))
	protected.Handle("POST /api/v1/customers", a.requireModule("crm", a.requirePermission("customers.write", http.HandlerFunc(a.createCustomer))))
	protected.Handle("GET /api/v1/customers/{id}", a.requirePermission("customers.read", http.HandlerFunc(a.getCustomer)))
	protected.Handle("PATCH /api/v1/customers/{id}", a.requirePermission("customers.write", http.HandlerFunc(a.updateCustomer)))
	protected.Handle("DELETE /api/v1/customers/{id}", a.requireRoles(http.HandlerFunc(a.deleteCustomer), "company_owner"))
	protected.Handle("GET /api/v1/customers/{id}/history", a.requirePermission("customers.read", http.HandlerFunc(a.customerAdminHistory)))
	protected.Handle("GET /api/v1/customers/{id}/timeline", a.requirePermission("customers.read", http.HandlerFunc(a.customerTimeline)))
	protected.Handle("GET /api/v1/customers/{id}/risk", a.requirePermission("customers.read", http.HandlerFunc(a.customerRisk)))
	protected.Handle("GET /api/v1/customers/{id}/rewards", a.requireModule("loyalty", a.requirePermission("customers.read", http.HandlerFunc(a.customerRewards))))
	protected.Handle("PATCH /api/v1/rewards/{id}", a.requireModule("loyalty", a.requirePermission("rewards.write", http.HandlerFunc(a.updateReward))))
	protected.Handle("GET /api/v1/reward-definitions", a.requireModule("loyalty", a.requirePermission("rewards.read", http.HandlerFunc(a.listRewardDefinitions))))
	protected.Handle("POST /api/v1/reward-definitions", a.requireModule("loyalty", a.requirePermission("rewards.manage", http.HandlerFunc(a.createRewardDefinition))))
	protected.Handle("PATCH /api/v1/reward-definitions/{id}", a.requireModule("loyalty", a.requirePermission("rewards.manage", http.HandlerFunc(a.updateRewardDefinition))))
	protected.Handle("DELETE /api/v1/reward-definitions/{id}", a.requireModule("loyalty", a.requirePermission("rewards.manage", http.HandlerFunc(a.deleteRewardDefinition))))
	protected.Handle("GET /api/v1/reward-rules", a.requireModule("loyalty", a.requirePermission("rewards.read", http.HandlerFunc(a.listRewardRules))))
	protected.Handle("POST /api/v1/reward-rules", a.requireModule("loyalty", a.requirePermission("rewards.manage", http.HandlerFunc(a.createRewardRule))))
	protected.Handle("PATCH /api/v1/reward-rules/{id}", a.requireModule("loyalty", a.requirePermission("rewards.manage", http.HandlerFunc(a.updateRewardRule))))
	protected.Handle("GET /api/v1/customers/{id}/reward-progress", a.requireModule("loyalty", a.requirePermission("customers.read", http.HandlerFunc(a.customerRewardProgress))))
	protected.Handle("POST /api/v1/rewards/issue", a.requireModule("loyalty", a.requirePermission("rewards.write", http.HandlerFunc(a.issueReward))))
	protected.Handle("POST /api/v1/rewards/{id}/reserve", a.requireModule("loyalty", a.requirePermission("rewards.write", http.HandlerFunc(a.reserveReward))))
	protected.Handle("POST /api/v1/rewards/{id}/redeem", a.requireModule("loyalty", a.requirePermission("rewards.write", http.HandlerFunc(a.redeemReward))))
	protected.Handle("POST /api/v1/rewards/{id}/cancel", a.requireModule("loyalty", a.requirePermission("rewards.write", http.HandlerFunc(a.cancelReward))))
	protected.Handle("GET /api/v1/rewards/{id}/transactions", a.requireModule("loyalty", a.requirePermission("rewards.read", http.HandlerFunc(a.rewardTransactions))))
	protected.Handle("POST /api/v1/rewards/expire", a.requireModule("loyalty", a.requirePermission("rewards.manage", http.HandlerFunc(a.expireRewards))))
	protected.Handle("POST /api/v1/customers/{id}/bonus", a.requireModule("loyalty", a.requirePermission("bonus.write", http.HandlerFunc(a.customerBonus))))
	protected.Handle("POST /api/v1/visits", a.requireModule("loyalty", a.requirePermission("visits.write", http.HandlerFunc(a.createVisit))))
	protected.Handle("GET /api/v1/visits", a.requirePermission("visits.read", http.HandlerFunc(a.listVisits)))
	protected.Handle("GET /api/v1/branches", a.requireRoles(http.HandlerFunc(a.listBranches), "company_owner", "employee"))
	protected.Handle("GET /api/v1/branches/{id}", a.requireRoles(http.HandlerFunc(a.branchDetail), "company_owner", "employee"))
	protected.Handle("POST /api/v1/branches", a.requireRoles(http.HandlerFunc(a.createBranch), "company_owner"))
	protected.Handle("PATCH /api/v1/branches/{id}", a.requireRoles(http.HandlerFunc(a.updateBranch), "company_owner"))
	protected.Handle("DELETE /api/v1/branches/{id}", a.requireRoles(http.HandlerFunc(a.deleteBranch), "company_owner"))
	protected.Handle("GET /api/v1/modules", a.requireRoles(http.HandlerFunc(a.listModules), "company_owner", "employee"))
	protected.Handle("PATCH /api/v1/modules/{code}", a.requireRoles(http.HandlerFunc(a.updateModule), "super_admin"))
	protected.Handle("GET /api/v1/loyalty/rules", a.requireRoles(http.HandlerFunc(a.getLoyaltyRules), "company_owner", "employee"))
	protected.Handle("PATCH /api/v1/loyalty/rules", a.requireRoles(http.HandlerFunc(a.updateLoyaltyRules), "company_owner"))
	protected.Handle("POST /api/v1/loyalty/process-birthdays", a.requireRoles(http.HandlerFunc(a.processBirthdays), "company_owner"))
	protected.Handle("GET /api/v1/loyalty/inactive", a.requireRoles(http.HandlerFunc(a.inactiveCustomers), "company_owner", "employee"))
	protected.Handle("GET /api/v1/referrals/program", a.requireRoles(http.HandlerFunc(a.getReferralProgram), "company_owner", "employee"))
	protected.Handle("PUT /api/v1/referrals/program", a.requireRoles(http.HandlerFunc(a.saveReferralProgram), "company_owner"))
	protected.Handle("GET /api/v1/referrals/analytics", a.requireRoles(http.HandlerFunc(a.referralAnalytics), "company_owner", "employee"))
	protected.Handle("GET /api/v1/employees", a.requireRoles(http.HandlerFunc(a.listEmployees), "company_owner"))
	protected.Handle("POST /api/v1/employees", a.requireRoles(http.HandlerFunc(a.createEmployee), "company_owner"))
	protected.Handle("PATCH /api/v1/employees/{id}", a.requireRoles(http.HandlerFunc(a.updateEmployee), "company_owner"))
	protected.Handle("DELETE /api/v1/employees/{id}", a.requireRoles(http.HandlerFunc(a.deleteEmployee), "company_owner"))
	protected.Handle("GET /api/v1/subscription", a.requireRoles(http.HandlerFunc(a.getSubscription), "company_owner"))
	protected.Handle("GET /api/v1/devices", a.requirePermission("devices.read", http.HandlerFunc(a.listDevices)))
	protected.Handle("POST /api/v1/devices", a.requireRoles(http.HandlerFunc(a.createDevice), "company_owner"))
	protected.Handle("PATCH /api/v1/devices/{id}", a.requireRoles(http.HandlerFunc(a.updateDevice), "company_owner"))
	protected.Handle("DELETE /api/v1/devices/{id}", a.requireRoles(http.HandlerFunc(a.deleteDevice), "company_owner"))
	protected.Handle("GET /api/v1/customer/me", a.requireRoles(http.HandlerFunc(a.customerMe), "customer"))
	protected.Handle("PATCH /api/v1/customer/me", a.requireRoles(http.HandlerFunc(a.updateCustomerProfile), "customer"))
	protected.Handle("GET /api/v1/customer/history", a.requireRoles(http.HandlerFunc(a.customerHistory), "customer"))
	protected.Handle("GET /api/v1/customer/rewards", a.requireModule("loyalty", a.requireRoles(http.HandlerFunc(a.customerOwnRewards), "customer")))
	protected.Handle("GET /api/v1/customer/wallet", a.requireRoles(http.HandlerFunc(a.customerWallet), "customer"))
	protected.Handle("POST /api/v1/customer/wheel/spin", a.requireRoles(http.HandlerFunc(a.customerWheelSpin), "customer"))
	protected.Handle("GET /api/v1/settings/guest-portal", a.requireRoles(http.HandlerFunc(a.getGuestPortalSettings), "company_owner"))
	protected.Handle("PATCH /api/v1/settings/guest-portal", a.requireRoles(http.HandlerFunc(a.updateGuestPortalSettings), "company_owner"))
	protected.Handle("GET /api/v1/admin/dashboard", a.requireRoles(http.HandlerFunc(a.adminDashboard), "super_admin"))
	protected.Handle("GET /api/v1/admin/companies", a.requireRoles(http.HandlerFunc(a.adminCompanies), "super_admin"))
	protected.Handle("POST /api/v1/admin/companies", a.requireRoles(http.HandlerFunc(a.adminCreateCompany), "super_admin"))
	protected.Handle("POST /api/v1/admin/companies/provision", a.requireRoles(http.HandlerFunc(a.adminProvisionCompany), "super_admin"))
	protected.Handle("GET /api/v1/admin/plans", a.requireRoles(http.HandlerFunc(a.adminPlans), "super_admin"))
	protected.Handle("PATCH /api/v1/admin/plans/{code}", a.requireRoles(http.HandlerFunc(a.adminUpdatePlan), "super_admin"))
	protected.Handle("GET /api/v1/admin/users", a.requireRoles(http.HandlerFunc(a.adminUsers), "super_admin"))
	protected.Handle("GET /api/v1/admin/analytics", a.requireRoles(http.HandlerFunc(a.adminPlatformAnalytics), "super_admin"))
	protected.Handle("GET /api/v1/admin/customers", a.requireRoles(http.HandlerFunc(a.adminCustomers), "super_admin"))
	protected.Handle("GET /api/v1/admin/companies/{id}", a.requireRoles(http.HandlerFunc(a.adminCompanyDetail), "super_admin"))
	protected.Handle("PATCH /api/v1/admin/companies/{id}/subscription", a.requireRoles(http.HandlerFunc(a.adminUpdateSubscription), "super_admin"))
	protected.Handle("PATCH /api/v1/admin/companies/{id}/status", a.requireRoles(http.HandlerFunc(a.adminUpdateCompanyStatus), "super_admin"))
	protected.Handle("GET /api/v1/analytics", a.requirePermission("analytics.read", http.HandlerFunc(a.analytics)))
	protected.Handle("GET /api/v1/analytics/business", a.requirePermission("analytics.read", http.HandlerFunc(a.businessAnalytics)))
	protected.Handle("GET /api/v1/analytics/outcomes", a.requirePermission("analytics.read", http.HandlerFunc(a.businessOutcomes)))
	protected.Handle("GET /api/v1/analytics/bonus-liability", a.requirePermission("bonus_liability.read", http.HandlerFunc(a.bonusLiability)))
	protected.Handle("GET /api/v1/analytics/retention", a.requirePermission("analytics.read", http.HandlerFunc(a.retentionCohorts)))
	protected.Handle("POST /api/v1/analytics/refresh", a.requireRoles(http.HandlerFunc(a.refreshAnalytics), "company_owner"))
	protected.Handle("GET /api/v1/reports/schedules", a.requirePermission("analytics.read", http.HandlerFunc(a.listReportSchedules)))
	protected.Handle("POST /api/v1/reports/schedules", a.requireRoles(http.HandlerFunc(a.createReportSchedule), "company_owner"))
	protected.Handle("PATCH /api/v1/reports/schedules/{id}", a.requireRoles(http.HandlerFunc(a.updateReportSchedule), "company_owner"))
	protected.Handle("DELETE /api/v1/reports/schedules/{id}", a.requireRoles(http.HandlerFunc(a.deleteReportSchedule), "company_owner"))
	protected.Handle("POST /api/v1/reports/schedules/{id}/run", a.requireRoles(http.HandlerFunc(a.runReportSchedule), "company_owner"))
	protected.Handle("GET /api/v1/reports/runs", a.requirePermission("analytics.read", http.HandlerFunc(a.listReportRuns)))
	protected.Handle("POST /api/v1/reports/runs/{id}/retry", a.requireRoles(http.HandlerFunc(a.retryReportRun), "company_owner"))
	protected.Handle("GET /api/v1/reports/runs/{id}/download", a.requirePermission("analytics.read", http.HandlerFunc(a.downloadReportRun)))
	protected.Handle("POST /api/v1/loyalty/expire-bonuses", a.requireRoles(http.HandlerFunc(a.expireBonusLotsNow), "company_owner"))
	protected.Handle("GET /api/v1/audit", a.requireRoles(http.HandlerFunc(a.auditList), "company_owner", "super_admin"))
	protected.Handle("GET /api/v1/settings/company", a.requireRoles(http.HandlerFunc(a.getCompanySettings), "company_owner"))
	protected.Handle("PATCH /api/v1/settings/company", a.requireRoles(http.HandlerFunc(a.updateCompanySettings), "company_owner"))
	protected.Handle("GET /api/v1/reviews/settings", a.requireModule("reviews", a.requireRoles(http.HandlerFunc(a.getReviewSettings), "company_owner")))
	protected.Handle("PATCH /api/v1/reviews/settings", a.requireModule("reviews", a.requireRoles(http.HandlerFunc(a.updateReviewSettings), "company_owner")))
	protected.Handle("GET /api/v1/notifications", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.listNotifications), "company_owner", "employee")))
	protected.Handle("POST /api/v1/notifications/send", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.sendNotification), "company_owner")))
	protected.Handle("GET /api/v1/website", a.requireModule("website", a.requireRoles(http.HandlerFunc(a.getWebsite), "company_owner")))
	protected.Handle("PATCH /api/v1/website", a.requireModule("website", a.requireRoles(http.HandlerFunc(a.updateWebsite), "company_owner")))
	protected.Handle("GET /api/v1/bookings", a.requireModule("booking", a.requireRoles(http.HandlerFunc(a.listBookings), "company_owner", "employee")))
	protected.Handle("PATCH /api/v1/bookings/{id}", a.requireModule("booking", a.requireRoles(http.HandlerFunc(a.updateBooking), "company_owner", "employee")))
	protected.Handle("GET /api/v1/api-keys", a.requireModule("api", a.requireRoles(http.HandlerFunc(a.listAPIKeys), "company_owner")))
	protected.Handle("POST /api/v1/api-keys", a.requireModule("api", a.requireRoles(http.HandlerFunc(a.createAPIKey), "company_owner")))
	protected.Handle("DELETE /api/v1/api-keys/{id}", a.requireModule("api", a.requireRoles(http.HandlerFunc(a.revokeAPIKey), "company_owner")))
	protected.Handle("POST /api/v1/upload", a.requirePermission("files.manage", http.HandlerFunc(a.uploadFile)))
	protected.Handle("GET /api/v1/files", a.requirePermission("files.read", http.HandlerFunc(a.listFiles)))
	protected.Handle("DELETE /api/v1/files/{id}", a.requireRoles(http.HandlerFunc(a.deleteFile), "company_owner"))
	protected.Handle("GET /api/v1/settings/integrations", a.requireRoles(http.HandlerFunc(a.getIntegrations), "company_owner"))
	protected.Handle("PATCH /api/v1/settings/integrations", a.requireRoles(http.HandlerFunc(a.updateIntegrations), "company_owner"))
	protected.Handle("GET /api/v1/integration-connections", a.requirePermission("integrations.read", http.HandlerFunc(a.listIntegrationConnections)))
	protected.Handle("POST /api/v1/integration-connections", a.requirePermission("integrations.manage", http.HandlerFunc(a.createIntegrationConnection)))
	protected.Handle("POST /api/v1/integration-connections/{id}/sync", a.requirePermission("integrations.manage", http.HandlerFunc(a.syncIntegrationConnection)))
	protected.Handle("GET /api/v1/integration-connections/{id}/sync-status", a.requirePermission("integrations.read", http.HandlerFunc(a.integrationSyncStatus)))
	protected.Handle("POST /api/v1/integration-jobs/{id}/retry", a.requirePermission("integrations.manage", http.HandlerFunc(a.integrationJobRetry)))
	protected.Handle("GET /api/v1/integration-connections/{id}/location-mappings", a.requirePermission("integrations.read", http.HandlerFunc(a.integrationLocationMappings)))
	protected.Handle("PATCH /api/v1/integration-connections/{id}/location-mappings/{mappingId}", a.requirePermission("integrations.manage", http.HandlerFunc(a.updateIntegrationLocationMapping)))
	protected.Handle("GET /api/v1/integration-connections/{id}/customer-links", a.requirePermission("integrations.read", http.HandlerFunc(a.integrationCustomerLinks)))
	protected.Handle("PATCH /api/v1/integration-connections/{id}/customer-links/{linkId}", a.requirePermission("integrations.manage", http.HandlerFunc(a.updateIntegrationCustomerLink)))
	protected.Handle("GET /api/v1/integration-connections/{id}/reconciliations", a.requirePermission("integrations.read", http.HandlerFunc(a.integrationReconciliations)))
	protected.Handle("POST /api/v1/integration-connections/{id}/reconcile", a.requirePermission("integrations.manage", http.HandlerFunc(a.startIntegrationReconciliation)))
	protected.Handle("POST /api/v1/integration-connections/{id}/inbound-webhook", a.requirePermission("integrations.manage", http.HandlerFunc(a.createInboundWebhook)))
	protected.Handle("GET /api/v1/integration-connections/{id}/inbound-webhook", a.requirePermission("integrations.read", http.HandlerFunc(a.getInboundWebhook)))
	protected.Handle("POST /api/v1/webhook-endpoints", a.requirePermission("integrations.manage", http.HandlerFunc(a.createOutboundWebhook)))
	protected.Handle("GET /api/v1/webhook-deliveries", a.requirePermission("integrations.read", http.HandlerFunc(a.listWebhookDeliveries)))
	protected.Handle("GET /api/v1/campaigns", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.listCampaigns), "company_owner", "employee")))
	protected.Handle("GET /api/v1/campaigns/{id}/analytics", a.requireModule("email", a.requirePermission("campaigns.analytics", http.HandlerFunc(a.campaignAnalytics))))
	protected.Handle("POST /api/v1/campaigns/{id}/events", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.recordCampaignEvent), "company_owner")))
	protected.Handle("POST /api/v1/campaigns", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.createCampaign), "company_owner")))
	protected.Handle("GET /api/v1/campaigns/{id}/preview", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.previewCampaign), "company_owner")))
	protected.Handle("POST /api/v1/campaigns/{id}/send", a.requireModule("email", a.requireRoles(http.HandlerFunc(a.sendCampaign), "company_owner")))
	protected.Handle("GET /api/v1/campaign-automations", a.requirePermission("automations.read", http.HandlerFunc(a.listCampaignAutomations)))
	protected.Handle("PATCH /api/v1/campaign-automations/{id}", a.requirePermission("automations.manage", http.HandlerFunc(a.updateCampaignAutomation)))
	protected.Handle("POST /api/v1/campaign-automations/run", a.requirePermission("automations.manage", http.HandlerFunc(a.runCampaignAutomations)))
	protected.Handle("GET /api/v1/partnerships", a.requireModule("partnerships", a.requirePermission("partnerships.read", http.HandlerFunc(a.listPartnerships))))
	protected.Handle("POST /api/v1/partnerships", a.requireModule("partnerships", a.requirePermission("partnerships.manage", http.HandlerFunc(a.createPartnership))))
	protected.Handle("POST /api/v1/partnerships/{id}/approve", a.requireModule("partnerships", a.requirePermission("partnerships.manage", http.HandlerFunc(a.approvePartnership))))
	protected.Handle("POST /api/v1/partnerships/{id}/offers", a.requireModule("partnerships", a.requirePermission("partnerships.manage", http.HandlerFunc(a.createPartnershipOffer))))
	protected.Handle("POST /api/v1/partnership-offers/redeem", a.requireModule("partnerships", a.requirePermission("partnerships.manage", http.HandlerFunc(a.redeemPartnershipOffer))))
	mux.Handle("/api/v1/", a.authenticate(a.requireWritableSubscription(a.auditMutations(protected))))
	return recoverer(cors(requestID(a.observe(a.rateLimit(csrfProtection(mux))))))
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	ready := true
	if err := a.db.Ping(ctx); err != nil {
		checks["postgres"] = "unavailable"
		ready = false
	}
	if err := a.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unavailable"
		ready = false
	}
	if !ready {
		write(w, http.StatusServiceUnavailable, envelope{Success: false, Data: map[string]any{"status": "degraded", "checks": checks, "time": time.Now().UTC()}})
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"status": "ok", "checks": checks, "time": time.Now().UTC()}})
}

func (a *api) dashboard(w http.ResponseWriter, r *http.Request) {
	var customers, visitsToday, bonusIssued, bonusRedeemed, registrations, branches, employees, devices, scans, repeatCustomers, rewardsIssued int
	var programConfigured, rewardConfigured, testCustomer bool
	err := a.db.QueryRow(r.Context(), `SELECT count(*), count(*) FILTER (WHERE created_at::date=current_date) FROM customers WHERE company_id=$1 AND deleted_at IS NULL`, companyID(r)).Scan(&customers, &registrations)
	if err == nil {
		err = a.db.QueryRow(r.Context(), `SELECT count(*), coalesce(sum(points_added),0) FROM visits WHERE company_id=$1 AND created_at::date=current_date`, companyID(r)).Scan(&visitsToday, &bonusIssued)
	}
	if err == nil {
		err = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(amount),0) FROM bonus_ledger WHERE company_id=$1 AND operation='debit' AND created_at::date=current_date`, companyID(r)).Scan(&bonusRedeemed)
	}
	if err == nil {
		err = a.db.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM branches WHERE company_id=$1 AND deleted_at IS NULL),(SELECT count(*) FROM users WHERE company_id=$1 AND role IN('company_owner','employee') AND deleted_at IS NULL),(SELECT count(*) FROM devices WHERE company_id=$1),(SELECT coalesce(sum(scans_count),0) FROM devices WHERE company_id=$1)`, companyID(r)).Scan(&branches, &employees, &devices, &scans)
	}
	if err == nil {
		err = a.db.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM customers WHERE company_id=$1 AND deleted_at IS NULL AND total_visits>=2),(SELECT count(*) FROM customer_rewards WHERE company_id=$1 AND status IN('available','reserved','redeemed'))`, companyID(r)).Scan(&repeatCustomers, &rewardsIssued)
	}
	if err == nil {
		err = a.db.QueryRow(r.Context(), `SELECT
			EXISTS(SELECT 1 FROM company_settings WHERE company_id=$1 AND loyalty_reward_rule_id IS NOT NULL),
			EXISTS(SELECT 1 FROM reward_definitions WHERE company_id=$1 AND is_active AND deleted_at IS NULL),
			EXISTS(SELECT 1 FROM visits WHERE company_id=$1 AND reversed_at IS NULL)`, companyID(r)).Scan(&programConfigured, &rewardConfigured, &testCustomer)
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить dashboard")
		return
	}
	latestCustomers := []map[string]any{}
	rows, _ := a.db.Query(r.Context(), `SELECT id,first_name,last_name,phone,total_points,created_at FROM customers WHERE company_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 5`, companyID(r))
	if rows != nil {
		for rows.Next() {
			var id, first, last, phone string
			var points int
			var created time.Time
			if rows.Scan(&id, &first, &last, &phone, &points, &created) == nil {
				latestCustomers = append(latestCustomers, map[string]any{"id": id, "name": strings.TrimSpace(first + " " + last), "phone": phone, "points": points, "createdAt": created})
			}
		}
		rows.Close()
	}
	latestVisits := []map[string]any{}
	visitRows, _ := a.db.Query(r.Context(), `SELECT v.id,c.first_name,c.last_name,b.name,v.points_added,v.created_at FROM visits v JOIN customers c ON c.id=v.customer_id JOIN branches b ON b.id=v.branch_id WHERE v.company_id=$1 ORDER BY v.created_at DESC LIMIT 5`, companyID(r))
	if visitRows != nil {
		for visitRows.Next() {
			var id, first, last, branch string
			var points int
			var created time.Time
			if visitRows.Scan(&id, &first, &last, &branch, &points, &created) == nil {
				latestVisits = append(latestVisits, map[string]any{"id": id, "customer": strings.TrimSpace(first + " " + last), "branch": branch, "points": points, "createdAt": created})
			}
		}
		visitRows.Close()
	}
	conversion := 0.0
	if scans > 0 {
		conversion = float64(customers) / float64(scans) * 100
	}
	launched := programConfigured && rewardConfigured && devices > 0 && testCustomer
	write(w, 200, envelope{Success: true, Data: map[string]any{"customers": customers, "visitsToday": visitsToday, "bonusIssued": bonusIssued, "bonusRedeemed": bonusRedeemed, "registrations": registrations, "repeatCustomers": repeatCustomers, "rewardsIssued": rewardsIssued, "nfcConversion": conversion, "latestCustomers": latestCustomers, "latestVisits": latestVisits, "onboarding": map[string]bool{"program": programConfigured, "reward": rewardConfigured, "device": devices > 0, "testCustomer": testCustomer, "launched": launched}}})
}

func (a *api) listCustomers(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	level := strings.TrimSpace(r.URL.Query().Get("level"))
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	birthday := strings.TrimSpace(r.URL.Query().Get("birthday"))
	registeredFrom := strings.TrimSpace(r.URL.Query().Get("registeredFrom"))
	registeredTo := strings.TrimSpace(r.URL.Query().Get("registeredTo"))
	minPoints := clamp(parseInt(r.URL.Query().Get("minPoints"), 0), 0, 100000000)
	limit := clamp(parseInt(r.URL.Query().Get("limit"), 20), 1, 100)
	page := clamp(parseInt(r.URL.Query().Get("page"), 1), 1, 100000)
	orders := map[string]string{"createdAt": "created_at", "name": "last_name", "points": "total_points", "visits": "total_visits"}
	orderColumn := orders[r.URL.Query().Get("sort")]
	if orderColumn == "" {
		orderColumn = "created_at"
	}
	orderDirection := "DESC"
	if strings.EqualFold(r.URL.Query().Get("order"), "asc") {
		orderDirection = "ASC"
	}
	query := fmt.Sprintf(`SELECT id,first_name,last_name,phone,birthday,total_points,total_visits,level,created_at,count(*) OVER() FROM customers c WHERE company_id=$1 AND deleted_at IS NULL AND (first_name ILIKE $2 OR last_name ILIKE $2 OR phone ILIKE $2) AND ($3='' OR level=$3) AND total_points >= $4 AND ($5='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id AND v.branch_id=nullif($5,'')::uuid)) AND ($6='' OR ($6='today' AND extract(month from birthday)=extract(month from current_date) AND extract(day from birthday)=extract(day from current_date)) OR ($6='month' AND extract(month from birthday)=extract(month from current_date))) AND ($7='' OR created_at::date >= nullif($7,'')::date) AND ($8='' OR created_at::date <= nullif($8,'')::date) ORDER BY %s %s LIMIT $9 OFFSET $10`, orderColumn, orderDirection)
	rows, err := a.db.Query(r.Context(), query, companyID(r), search, level, minPoints, branch, birthday, registeredFrom, registeredTo, limit, (page-1)*limit)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить клиентов")
		return
	}
	defer rows.Close()
	items := []customer{}
	total := 0
	for rows.Next() {
		var c customer
		if err = rows.Scan(&c.ID, &c.FirstName, &c.LastName, &c.Phone, &c.Birthday, &c.TotalPoints, &c.TotalVisits, &c.Level, &c.CreatedAt, &total); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать клиентов")
			return
		}
		items = append(items, c)
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"items": items, "page": page, "limit": limit, "total": total, "pages": (total + limit - 1) / limit}})
}

func (a *api) createCustomer(w http.ResponseWriter, r *http.Request) {
	var in customerInput
	if !decode(w, r, &in) {
		return
	}
	if ok, limit := a.checkLimit(r.Context(), companyID(r), "customers"); !ok {
		fail(w, 409, "LIMIT_REACHED", fmt.Sprintf("Достигнут лимит клиентов: %d", limit.Used))
		return
	}
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.Phone = strings.TrimSpace(in.Phone)
	if in.FirstName == "" || len(in.Phone) < 7 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите имя и корректный телефон")
		return
	}
	if in.Email != "" && (!strings.Contains(in.Email, "@") || strings.ContainsAny(in.Email, "\r\n")) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите корректный Email")
		return
	}
	var birthday any
	if in.Birthday != "" {
		parsed, err := time.Parse("2006-01-02", in.Birthday)
		if err != nil {
			fail(w, 422, "VALIDATION_ERROR", "Некорректная дата рождения")
			return
		}
		birthday = parsed
	}
	var c customer
	err := a.db.QueryRow(r.Context(), `INSERT INTO customers(company_id,first_name,last_name,phone,email,birthday) VALUES($1,$2,$3,$4,nullif($5,''),$6) RETURNING id,first_name,last_name,phone,coalesce(email,''),birthday,total_points,total_visits,level,created_at`, companyID(r), in.FirstName, strings.TrimSpace(in.LastName), in.Phone, strings.TrimSpace(in.Email), birthday).Scan(&c.ID, &c.FirstName, &c.LastName, &c.Phone, &c.Email, &c.Birthday, &c.TotalPoints, &c.TotalVisits, &c.Level, &c.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "customers_company_phone_unique") {
			fail(w, 409, "CUSTOMER_EXISTS", "Клиент с таким телефоном уже существует")
			return
		}
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать клиента")
		return
	}
	write(w, 201, envelope{Success: true, Data: c})
}

func (a *api) getCustomer(w http.ResponseWriter, r *http.Request) {
	var c customer
	err := a.db.QueryRow(r.Context(), `SELECT c.id,c.first_name,c.last_name,c.phone,coalesce(c.email,''),c.birthday,c.total_points,c.total_visits,c.level,c.created_at,
		coalesce((SELECT b.name FROM visits v JOIN branches b ON b.id=v.branch_id WHERE v.company_id=c.company_id AND v.customer_id=c.id GROUP BY b.id,b.name ORDER BY count(*) DESC,max(v.created_at) DESC LIMIT 1),''),
		coalesce((SELECT b.name FROM visits v JOIN branches b ON b.id=v.branch_id WHERE v.company_id=c.company_id AND v.customer_id=c.id ORDER BY v.created_at DESC LIMIT 1),'')
		FROM customers c WHERE c.company_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, companyID(r), r.PathValue("id")).Scan(&c.ID, &c.FirstName, &c.LastName, &c.Phone, &c.Email, &c.Birthday, &c.TotalPoints, &c.TotalVisits, &c.Level, &c.CreatedAt, &c.FavoriteBranch, &c.LastBranch)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент не найден")
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить клиента")
		return
	}
	write(w, 200, envelope{Success: true, Data: c})
}

func (a *api) createVisit(w http.ResponseWriter, r *http.Request) {
	var in visitInput
	if !decode(w, r, &in) {
		return
	}
	if in.CustomerID == "" || in.BranchID == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите клиента и филиал")
		return
	}
	tenant := companyID(r)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var branchAllowed bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM branches b WHERE b.company_id=$1 AND b.id=$2 AND b.deleted_at IS NULL AND b.is_active AND ($3<>'employee' OR b.id=(SELECT branch_id FROM users WHERE id=$4 AND company_id=$1)))`, tenant, in.BranchID, claims.Role, claims.Subject).Scan(&branchAllowed); err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось проверить филиал")
		return
	}
	if !branchAllowed {
		fail(w, 403, "BRANCH_ACCESS_DENIED", "Сотрудник может проводить операции только в своём активном филиале")
		return
	}
	var balance, visits int
	err = tx.QueryRow(r.Context(), `SELECT total_points,total_visits FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, tenant, in.CustomerID).Scan(&balance, &visits)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент не найден")
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить клиента")
		return
	}
	var duplicate bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM visits WHERE company_id=$1 AND customer_id=$2 AND created_at>now()-interval '2 minutes')`, tenant, in.CustomerID).Scan(&duplicate); err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось проверить последнее посещение")
		return
	}
	if duplicate {
		_ = tx.Rollback(r.Context())
		a.recordRisk(r, in.CustomerID, in.BranchID, "visit.create", "blocked", "Повторное посещение в течение 2 минут", map[string]any{"windowMinutes": 2})
		fail(w, 409, "DUPLICATE_VISIT", "Посещение уже отмечено. Новое можно добавить через 2 минуты")
		return
	}
	var visitsToday int
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM visits WHERE company_id=$1 AND customer_id=$2 AND created_at>now()-interval '24 hours'`, tenant, in.CustomerID).Scan(&visitsToday); err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось проверить лимит посещений")
		return
	}
	if visitsToday >= 5 {
		_ = tx.Rollback(r.Context())
		a.recordRisk(r, in.CustomerID, in.BranchID, "visit.create", "blocked", "Превышен дневной лимит посещений", map[string]any{"visits24h": visitsToday})
		fail(w, 409, "VISIT_LIMIT_REACHED", "За 24 часа уже отмечено 5 посещений. Обратитесь к владельцу")
		return
	}
	points := a.pointsForEvent(r.Context(), tenant, "visit_created", 20)
	var visitID string
	err = tx.QueryRow(r.Context(), `INSERT INTO visits(company_id,branch_id,customer_id,employee_id,points_added,comment) SELECT $1,b.id,$3,$4,$5,$6 FROM branches b WHERE b.company_id=$1 AND b.id=$2 AND b.deleted_at IS NULL RETURNING id`, tenant, in.BranchID, in.CustomerID, claims.Subject, points, in.Comment).Scan(&visitID)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE customers SET total_visits=total_visits+1,total_points=total_points+$3,level=CASE WHEN total_visits+1>=50 THEN 'vip' WHEN total_visits+1>=25 THEN 'gold' WHEN total_visits+1>=10 THEN 'silver' ELSE 'basic' END,updated_at=now() WHERE company_id=$1 AND id=$2`, tenant, in.CustomerID, points)
	}
	if err == nil {
		var ledgerID string
		err = tx.QueryRow(r.Context(), `INSERT INTO bonus_ledger(company_id,customer_id,visit_id,operation,amount,balance_after,description) VALUES($1,$2,$3,'credit',$4,$5,'Бонус за посещение') RETURNING id`, tenant, in.CustomerID, visitID, points, balance+points).Scan(&ledgerID)
		if err == nil && points > 0 {
			err = posintegration.IssueBonusLot(r.Context(), tx, tenant, in.CustomerID, ledgerID, "", points)
		}
	}
	rewardName := ""
	if err == nil {
		var issued []string
		issued, err = a.evaluateRewardEvent(r, tx, tenant, in.CustomerID, "visit_created", visits+1)
		if len(issued) > 0 {
			rewardName = fmt.Sprintf("%d наград", len(issued))
		}
	}
	if err == nil {
		err = appendCustomerEvent(r, tx, tenant, in.CustomerID, "visit.completed", in.BranchID, "visit:"+visitID, map[string]any{"visitId": visitID, "pointsAdded": points, "totalVisits": visits + 1})
	}
	if err == nil && visits+1 == 2 {
		err = appendCustomerEvent(r, tx, tenant, in.CustomerID, "customer.returned", in.BranchID, "visit-return:"+visitID, map[string]any{"reason": "second_visit", "totalVisits": visits + 1})
	}
	if err == nil && points > 0 {
		err = appendCustomerEvent(r, tx, tenant, in.CustomerID, "bonus.earned", in.BranchID, "visit-bonus:"+visitID, map[string]any{"amount": points, "balanceAfter": balance + points, "reason": "Бонус за посещение"})
	}
	if err != nil {
		slog.Error("loyalty.visit.failed", "event_type", "loyalty.visit.failed", "tenant_id", tenant, "actor_id", identity(r).Subject, "customer_id", in.CustomerID, "request_id", r.Header.Get("X-Request-ID"), "error", err)
		fail(w, 500, "VISIT_FAILED", "Не удалось добавить посещение")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "VISIT_FAILED", "Не удалось сохранить посещение")
		return
	}
	logDomainEvent(r, "loyalty.visit.recorded", in.CustomerID, "visit_id", visitID, "branch_id", in.BranchID, "points_added", points)
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": visitID, "pointsAdded": points, "balance": balance + points, "totalVisits": visits + 1, "reward": rewardName}})
}

func (a *api) listBranches(w http.ResponseWriter, r *http.Request) {
	claims := identity(r)
	rows, err := a.db.Query(r.Context(), `SELECT b.id,b.name,b.address,coalesce(b.phone,''),b.is_active FROM branches b WHERE b.company_id=$1 AND b.deleted_at IS NULL AND ($2<>'employee' OR b.id=(SELECT branch_id FROM users WHERE id=$3 AND company_id=$1)) ORDER BY b.name`, companyID(r), claims.Role, claims.Subject)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить филиалы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, address, phone string
		var active bool
		if rows.Scan(&id, &name, &address, &phone, &active) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "address": address, "phone": phone, "active": active})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) listModules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT m.code,m.name,m.is_core,coalesce(cm.enabled,false) FROM modules m LEFT JOIN company_modules cm ON cm.module_code=m.code AND cm.company_id=$1 ORDER BY m.is_core DESC,m.name`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить модули")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var code, name string
		var core, enabled bool
		if rows.Scan(&code, &name, &core, &enabled) == nil {
			if code == "analytics" {
				name = "Расширенная аналитика"
			}
			included := a.moduleIncluded(r.Context(), companyID(r), code)
			requiredPlan := "Growth"
			if code == "api" {
				requiredPlan = "Pro"
			} else if included && (core || code == "crm" || code == "loyalty" || code == "reviews") {
				requiredPlan = "Starter"
			}
			items = append(items, map[string]any{"code": code, "name": name, "core": core, "enabled": included && (enabled || core), "available": included, "requiredPlan": requiredPlan})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		fail(w, 400, "INVALID_JSON", "Некорректный JSON")
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("API-Version", "1")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	write(w, status, envelope{Success: false, Error: &apiError{Code: code, Message: message}})
}
func parseInt(v string, d int) int {
	n, e := strconv.Atoi(v)
	if e != nil {
		return d
	}
	return n
}
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := envOr("WEB_ORIGIN", "http://localhost:3000")
		if origin == allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = strconv.FormatInt(time.Now().UnixNano(), 36)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				fail(w, 500, "INTERNAL_ERROR", "Внутренняя ошибка сервера")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

var _ = context.Background
