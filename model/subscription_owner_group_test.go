package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNormalizeOwnerGroup(t *testing.T) {
	assert.Equal(t, "vip,svip", NormalizeOwnerGroup(" vip,svip,vip, ,"))
	assert.Empty(t, NormalizeOwnerGroup(" , , "))
	assert.Equal(t, []string{"vip", "svip"}, OwnerGroupList(" vip, svip "))
}

// Database matrix uses isolated tables, never deployment subscription tables.
func TestOwnerGroupDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "owner_group_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			t.Cleanup(func() {
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				assert.NoError(t, db.Migrator().DropTable(&SubscriptionPreConsumeRecord{}, &UserSubscription{}, &SubscriptionPlan{}))
				assert.NoError(t, sqlDB.Close())
			})
			versionSQL := "SELECT version()"
			if dialect == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("database version: %s", version)
			require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}))
			now := common.GetTimestamp()
			plan := SubscriptionPlan{Id: 981001, Title: "Group plan", DurationUnit: "month", DurationValue: 1, OwnerGroup: "vip,svip", TotalAmount: 1000, QuotaResetPeriod: "never"}
			require.NoError(t, db.Create(&plan).Error)
			InvalidateSubscriptionPlanCache(plan.Id)
			t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
			sub := UserSubscription{UserId: 981001, PlanId: plan.Id, Status: "active", EndTime: now + 3600, AmountTotal: 1000, AmountUsed: 70, OwnerGroup: "vip,svip", AllowWalletOverflow: true}
			require.NoError(t, db.Create(&sub).Error)
			cancelled := UserSubscription{UserId: sub.UserId, PlanId: plan.Id, Status: "cancelled", EndTime: now + 3600, OwnerGroup: "legacy"}
			require.NoError(t, db.Create(&cancelled).Error)
			// The latest upstream schema has no owner_group column.
			require.NoError(t, db.Migrator().DropColumn(&UserSubscription{}, "OwnerGroup"))
			for range 2 {
				require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}))
				require.NoError(t, backfillUserSubscriptionOwnerGroup())
			}
			var restored, inactive UserSubscription
			require.NoError(t, db.First(&restored, sub.Id).Error)
			assert.Equal(t, "vip,svip", restored.OwnerGroup)
			assert.EqualValues(t, 70, restored.AmountUsed)
			assert.Equal(t, sub.EndTime, restored.EndTime)
			require.NoError(t, db.First(&inactive, cancelled.Id).Error)
			assert.Empty(t, inactive.OwnerGroup)
			for i, tc := range []struct {
				owner, group string
				allowed      bool
			}{
				{"", "vip", true}, {"vip,svip", "vip", true}, {"vip,svip", "svip", true}, {"a,vip,b", "vip", true},
				{"vipplus,svip", "vip", false}, {"vip,svip", "", false}, {"", "", true},
				{"a,team_x", "team_x", true}, {"a,teamax", "team_x", false},
				{"a,team%", "team%", true}, {"a,teamXYZ", "team%", false}, {"a,team!", "team!", true},
				{"VIP,svip", "vip", false}, {"a,VIP,b", "vip", false},
			} {
				t.Run(fmt.Sprintf("matching_%d", i), func(t *testing.T) {
					require.NoError(t, db.Model(&sub).Update("owner_group", tc.owner).Error)
					allowed, err := HasActiveUserSubscription(sub.UserId, tc.group)
					require.NoError(t, err)
					assert.Equal(t, tc.allowed, allowed)
					if !tc.allowed {
						_, err = PreConsumeUserSubscription(fmt.Sprintf("denied-%d", i), sub.UserId, "model", 0, 1, tc.group)
						require.Error(t, err)
					}
				})
			}
			require.NoError(t, db.Model(&sub).Update("owner_group", "vip,svip").Error)
			strict := UserSubscription{UserId: sub.UserId, PlanId: plan.Id, Status: "active", EndTime: now + 3600, OwnerGroup: "other", AllowWalletOverflow: false}
			require.NoError(t, db.Create(&strict).Error)
			allowed, err := UserActiveSubscriptionsAllowWalletOverflow(sub.UserId, "vip")
			require.NoError(t, err)
			assert.True(t, allowed)
			allowed, err = UserActiveSubscriptionsAllowWalletOverflow(sub.UserId, "other")
			require.NoError(t, err)
			assert.False(t, allowed)
			allowed, err = UserActiveSubscriptionsAllowWalletOverflow(sub.UserId, "OTHER")
			require.NoError(t, err)
			assert.True(t, allowed)
			_, err = PreConsumeUserSubscription("owner-denied", sub.UserId, "model", 0, 100, "absent")
			require.Error(t, err)
			result, err := PreConsumeUserSubscription("owner-allowed", sub.UserId, "model", 0, 100, "svip")
			require.NoError(t, err)
			assert.Equal(t, sub.Id, result.UserSubscriptionId)
			assert.Equal(t, "vip,svip", result.OwnerGroup)
			_, err = PreConsumeUserSubscription("owner-allowed", sub.UserId, "model", 0, 100, "absent")
			require.Error(t, err)
			require.NoError(t, RefundSubscriptionPreConsume("owner-allowed"))
			require.NoError(t, RefundSubscriptionPreConsume("owner-allowed"))
			require.NoError(t, db.First(&restored, sub.Id).Error)
			assert.EqualValues(t, 70, restored.AmountUsed)
		})
	}
}
