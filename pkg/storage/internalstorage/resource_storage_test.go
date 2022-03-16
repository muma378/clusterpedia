package internalstorage

import (
	"context"
	"fmt"
	"testing"

	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	internal "github.com/clusterpedia-io/clusterpedia/pkg/apis/clusterpedia"
)

func testApplyListOptionsToResourceQuery(t *testing.T, name string, options *internal.ListOptions, expected expected) {
	t.Run(fmt.Sprintf("%s postgres", name), func(t *testing.T) {
		postgreSQL, err := toSQL(postgresDB, options,
			func(query *gorm.DB, options *internal.ListOptions) (*gorm.DB, error) {
				_, _, query, err := applyListOptionsToResourceQuery(postgresDB, query, options)
				return query, err
			},
		)

		assertError(t, expected.err, err)
		if postgreSQL != expected.postgres {
			t.Errorf("expected sql: %q, but got: %q", expected.postgres, postgreSQL)
		}
	})

	for version := range mysqlDBs {
		t.Run(fmt.Sprintf("%s mysql-%s", name, version), func(t *testing.T) {
			mysqlDB := mysqlDBs[version]
			mysqlSQL, err := toSQL(mysqlDB, options,
				func(query *gorm.DB, options *internal.ListOptions) (*gorm.DB, error) {
					_, _, query, err := applyListOptionsToResourceQuery(mysqlDB, query, options)
					return query, err
				},
			)

			assertError(t, expected.err, err)
			if mysqlSQL != expected.mysql {
				t.Errorf("expected sql: %q, but got: %q", expected.mysql, mysqlSQL)
			}
		})
	}
}

func TestApplyListOptionsToResourceQuery_Owner(t *testing.T) {
	tests := []struct {
		name        string
		listOptions *internal.ListOptions
		expected    expected
	}{
		{
			"owner uid",
			&internal.ListOptions{
				ClusterNames: []string{"cluster-1"},
				OwnerUID:     "owner-uid-1",
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND owner_uid = 'owner-uid-1' `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND owner_uid = 'owner-uid-1' ",
				"",
			},
		},
		{
			"owner uid with seniority",
			&internal.ListOptions{
				ClusterNames:   []string{"cluster-1"},
				OwnerUID:       "owner-uid-1",
				OwnerSeniority: 1,
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND owner_uid IN (SELECT "uid" FROM "resources" WHERE "cluster" = 'cluster-1' AND owner_uid = 'owner-uid-1') `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND owner_uid IN (SELECT `uid` FROM `resources` WHERE `cluster` = 'cluster-1' AND owner_uid = 'owner-uid-1') ",
				"",
			},
		},
		{
			"owner name",
			&internal.ListOptions{
				ClusterNames: []string{"cluster-1"},
				OwnerName:    "owner-name-1",
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND owner_uid IN (SELECT "uid" FROM "resources" WHERE "cluster" = 'cluster-1' AND name = 'owner-name-1') `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND owner_uid IN (SELECT `uid` FROM `resources` WHERE `cluster` = 'cluster-1' AND name = 'owner-name-1') ",
				"",
			},
		},
		{
			"owner uid and name",
			&internal.ListOptions{
				ClusterNames: []string{"cluster-1"},
				OwnerUID:     "owner-uid-1",
				OwnerName:    "owner_name-1",
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND owner_uid = 'owner-uid-1' `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND owner_uid = 'owner-uid-1' ",
				"",
			},
		},
		{
			"owner name with group resource",
			&internal.ListOptions{
				ClusterNames:       []string{"cluster-1"},
				OwnerName:          "owner-name-1",
				OwnerGroupResource: schema.GroupResource{Group: "apps", Resource: "deployments"},
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND owner_uid IN (SELECT "uid" FROM "resources" WHERE "cluster" = 'cluster-1' AND "group" = 'apps' AND "resource" = 'deployments' AND name = 'owner-name-1') `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND owner_uid IN (SELECT `uid` FROM `resources` WHERE `cluster` = 'cluster-1' AND `group` = 'apps' AND `resource` = 'deployments' AND name = 'owner-name-1') ",
				"",
			},
		},
		{
			"only owner group resource and seniroty",
			&internal.ListOptions{
				ClusterNames:       []string{"cluster-1"},
				OwnerSeniority:     1,
				OwnerGroupResource: schema.GroupResource{Group: "apps", Resource: "deployments"},
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' ",
				"",
			},
		},

		// with clusters
		{
			"with multi clusters",
			&internal.ListOptions{
				ClusterNames: []string{"cluster-1", "cluster-2"},
				OwnerUID:     "owner-uid-1",
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster IN ('cluster-1','cluster-2') `,
				"SELECT * FROM `resources` WHERE cluster IN ('cluster-1','cluster-2') ",
				"",
			},
		},

		// with namespaces
		{
			"owner uid with namespaces",
			&internal.ListOptions{
				ClusterNames:   []string{"cluster-1"},
				Namespaces:     []string{"ns-1", "ns-2"},
				OwnerUID:       "owner-uid-1",
				OwnerSeniority: 1,
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND namespace IN ('ns-1','ns-2') AND owner_uid IN (SELECT "uid" FROM "resources" WHERE "cluster" = 'cluster-1' AND owner_uid = 'owner-uid-1') `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND namespace IN ('ns-1','ns-2') AND owner_uid IN (SELECT `uid` FROM `resources` WHERE `cluster` = 'cluster-1' AND owner_uid = 'owner-uid-1') ",
				"",
			},
		},
		{
			"owner name with namespaces",
			&internal.ListOptions{
				ClusterNames: []string{"cluster-1"},
				Namespaces:   []string{"ns-1", "ns-2"},
				OwnerName:    "owner-name-1",
			},
			expected{
				`SELECT * FROM "resources" WHERE cluster = 'cluster-1' AND namespace IN ('ns-1','ns-2') AND owner_uid IN (SELECT "uid" FROM "resources" WHERE "cluster" = 'cluster-1' AND namespace IN ('ns-1','ns-2','') AND name = 'owner-name-1') `,
				"SELECT * FROM `resources` WHERE cluster = 'cluster-1' AND namespace IN ('ns-1','ns-2') AND owner_uid IN (SELECT `uid` FROM `resources` WHERE `cluster` = 'cluster-1' AND namespace IN ('ns-1','ns-2','') AND name = 'owner-name-1') ",
				"",
			},
		},
	}

	for _, test := range tests {
		testApplyListOptionsToResourceQuery(t, test.name, test.listOptions, test.expected)
	}
}

func TestResourceStorage_genGetObjectQuery(t *testing.T) {
	tests := []struct {
		name         string
		resource     schema.GroupVersionResource
		cluster      string
		namespace    string
		resourceName string
		expected     expected
	}{
		{
			"empty",
			schema.GroupVersionResource{},
			"",
			"",
			"",
			expected{
				`SELECT "object" FROM "resources" WHERE "cluster" = '' AND "group" = '' AND "name" = '' AND "namespace" = '' AND "resource" = '' AND "version" = '' ORDER BY "resources"."id" LIMIT 1`,
				"SELECT `object` FROM `resources` WHERE `cluster` = '' AND `group` = '' AND `name` = '' AND `namespace` = '' AND `resource` = '' AND `version` = '' ORDER BY `resources`.`id` LIMIT 1",
				"",
			},
		},
		{
			"non empty",
			appsv1.SchemeGroupVersion.WithResource("deployments"),
			"cluster-1",
			"ns-1",
			"resource-1",
			expected{
				`SELECT "object" FROM "resources" WHERE "cluster" = 'cluster-1' AND "group" = 'apps' AND "name" = 'resource-1' AND "namespace" = 'ns-1' AND "resource" = 'deployments' AND "version" = 'v1' ORDER BY "resources"."id" LIMIT 1`,
				"SELECT `object` FROM `resources` WHERE `cluster` = 'cluster-1' AND `group` = 'apps' AND `name` = 'resource-1' AND `namespace` = 'ns-1' AND `resource` = 'deployments' AND `version` = 'v1' ORDER BY `resources`.`id` LIMIT 1",
				"",
			},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s postgres", test.name), func(t *testing.T) {
			postgreSQL := postgresDB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				rs := newTestResourceStorage(tx, test.resource)
				return rs.genGetObjectQuery(context.TODO(), test.cluster, test.namespace, test.resourceName).First(interface{}(nil))
			})

			if postgreSQL != test.expected.postgres {
				t.Errorf("expected sql: %q, but got: %q", test.expected.postgres, postgreSQL)
			}
		})

		for version := range mysqlDBs {
			t.Run(fmt.Sprintf("%s mysql-%s", test.name, version), func(t *testing.T) {
				mysqlSQL := mysqlDBs[version].ToSQL(func(tx *gorm.DB) *gorm.DB {
					rs := newTestResourceStorage(tx, test.resource)
					return rs.genGetObjectQuery(context.TODO(), test.cluster, test.namespace, test.resourceName).First(interface{}(nil))
				})

				if mysqlSQL != test.expected.mysql {
					t.Errorf("expected sql: %q, but got: %q", test.expected.mysql, mysqlSQL)
				}
			})
		}
	}
}

func TestResourceStorage_genListObjectQuery(t *testing.T) {
	tests := []struct {
		name        string
		resource    schema.GroupVersionResource
		listOptions *internal.ListOptions
		expected    expected
	}{
		{
			"empty list options",
			appsv1.SchemeGroupVersion.WithResource("deployments"),
			&internal.ListOptions{},
			expected{
				`SELECT "object" FROM "resources" WHERE "group" = 'apps' AND "resource" = 'deployments' AND "version" = 'v1' `,
				"SELECT `object` FROM `resources` WHERE `group` = 'apps' AND `resource` = 'deployments' AND `version` = 'v1' ",
				"",
			},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s postgres", test.name), func(t *testing.T) {
			postgreSQL, err := toSQL(postgresDB.Session(&gorm.Session{DryRun: true}), test.listOptions,
				func(db *gorm.DB, options *internal.ListOptions) (*gorm.DB, error) {
					rs := newTestResourceStorage(db, test.resource)
					_, _, query, err := rs.genListObjectsQuery(context.TODO(), options)
					return query, err
				},
			)

			assertError(t, test.expected.err, err)
			if postgreSQL != test.expected.postgres {
				t.Errorf("expected sql: %q, but got: %q", test.expected.postgres, postgreSQL)
			}
		})

		for version := range mysqlDBs {
			t.Run(fmt.Sprintf("%s mysql-%s", test.name, version), func(t *testing.T) {
				mysqlSQL, err := toSQL(mysqlDBs[version].Session(&gorm.Session{DryRun: true}), test.listOptions,
					func(db *gorm.DB, options *internal.ListOptions) (*gorm.DB, error) {
						rs := newTestResourceStorage(db, test.resource)
						_, _, query, err := rs.genListObjectsQuery(context.TODO(), options)
						return query, err
					},
				)

				assertError(t, test.expected.err, err)
				if mysqlSQL != test.expected.mysql {
					t.Errorf("expected sql: %q, but got: %q", test.expected.mysql, mysqlSQL)
				}
			})
		}
	}
}

func TestResourceStorage_deleteObject(t *testing.T) {
	tests := []struct {
		name         string
		resource     schema.GroupVersionResource
		cluster      string
		namespace    string
		resourceName string
		expected     expected
	}{
		{
			"empty",
			appsv1.SchemeGroupVersion.WithResource("deployments"),
			"",
			"",
			"",
			expected{
				`DELETE FROM "resources" WHERE "cluster" = '' AND "group" = 'apps' AND "name" = '' AND "namespace" = '' AND "resource" = 'deployments' AND "version" = 'v1'`,
				"DELETE FROM `resources` WHERE `cluster` = '' AND `group` = 'apps' AND `name` = '' AND `namespace` = '' AND `resource` = 'deployments' AND `version` = 'v1'",
				"",
			},
		},
		{
			"non empty",
			appsv1.SchemeGroupVersion.WithResource("deployments"),
			"cluster-1",
			"ns-1",
			"resource-1",
			expected{
				`DELETE FROM "resources" WHERE "cluster" = 'cluster-1' AND "group" = 'apps' AND "name" = 'resource-1' AND "namespace" = 'ns-1' AND "resource" = 'deployments' AND "version" = 'v1'`,
				"DELETE FROM `resources` WHERE `cluster` = 'cluster-1' AND `group` = 'apps' AND `name` = 'resource-1' AND `namespace` = 'ns-1' AND `resource` = 'deployments' AND `version` = 'v1'",
				"",
			},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s postgres", test.name), func(t *testing.T) {
			// If SkipDefaultTransaction is not set to true, will case
			// 'all expectations were already fulfilled, call to database transaction Begin was not expected'
			postgreSQL := postgresDB.Session(&gorm.Session{SkipDefaultTransaction: true}).ToSQL(
				func(tx *gorm.DB) *gorm.DB {
					rs := newTestResourceStorage(tx, test.resource)
					return rs.deleteObject(test.cluster, test.namespace, test.resourceName)
				})

			if postgreSQL != test.expected.postgres {
				t.Errorf("expected sql: %q, but got: %q", test.expected.postgres, postgreSQL)
			}
		})

		for version := range mysqlDBs {
			t.Run(fmt.Sprintf("%s mysql-%s", test.name, version), func(t *testing.T) {
				// If SkipDefaultTransaction is not set to true, will case
				// 'all expectations were already fulfilled, call to database transaction Begin was not expected'
				mysqlSQL := mysqlDBs[version].Session(&gorm.Session{SkipDefaultTransaction: true}).ToSQL(
					func(tx *gorm.DB) *gorm.DB {
						rs := newTestResourceStorage(tx, test.resource)
						return rs.deleteObject(test.cluster, test.namespace, test.resourceName)
					})

				if mysqlSQL != test.expected.mysql {
					t.Errorf("expected sql: %q, but got: %q", test.expected.mysql, mysqlSQL)
				}
			})
		}
	}
}

func newTestResourceStorage(db *gorm.DB, storageGVK schema.GroupVersionResource) *ResourceStorage {
	return &ResourceStorage{
		db:                   db,
		storageGroupResource: storageGVK.GroupResource(),
		storageVersion:       storageGVK.GroupVersion(),
	}
}
