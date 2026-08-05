//go:build ignore

// Load test cleanup.
//
//	cd backend
//	go run scripts/cleanup_loadtest.go --subdomain loadtest1754...      # dry run
//	go run scripts/cleanup_loadtest.go --subdomain loadtest1754... --confirm
//	go run scripts/cleanup_loadtest.go --all                            # dry run, every load-test tenant
//
// Every tenant-scoped table cascades from tenants(id), so removing the
// tenant row removes its products, orders, tickets, campaigns and AI
// logs with it.
//
// Two safety rails, because this runs against the same database the
// live demo uses:
//
//  1. It refuses any subdomain that does not start with "loadtest".
//     The load script only ever creates subdomains in that namespace.
//  2. It prints what it would delete and exits. Nothing is removed
//     without --confirm.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// loadTestPrefix is the namespace the load script creates tenants in.
// Keep in sync with scripts/test/load_checkout.js.
const loadTestPrefix = "loadtest"

type tenant struct {
	id        string
	subdomain string
	name      string
	createdAt time.Time
	orders    int
	products  int
}

func main() {
	subdomain := flag.String("subdomain", "", "exact subdomain to remove (must start with \"loadtest\")")
	all := flag.Bool("all", false, "target every tenant whose subdomain starts with \"loadtest\"")
	confirm := flag.Bool("confirm", false, "actually delete; without it this is a dry run")
	flag.Parse()

	if *subdomain == "" && !*all {
		fmt.Fprintln(os.Stderr, "specify --subdomain <name> or --all")
		flag.Usage()
		os.Exit(2)
	}
	if *subdomain != "" && !strings.HasPrefix(*subdomain, loadTestPrefix) {
		fmt.Fprintf(os.Stderr,
			"refusing to touch %q: this script only removes tenants whose subdomain starts with %q\n",
			*subdomain, loadTestPrefix)
		os.Exit(1)
	}

	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	targets, err := findTargets(ctx, db, *subdomain, *all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find targets: %v\n", err)
		os.Exit(1)
	}

	if len(targets) == 0 {
		fmt.Println("Nothing to clean up.")
		return
	}

	fmt.Printf("Load-test tenants found: %d\n\n", len(targets))
	var totalOrders, totalProducts int
	for _, t := range targets {
		fmt.Printf("  %s  (%s)\n", t.subdomain, t.name)
		fmt.Printf("    created  %s\n", t.createdAt.Format(time.RFC3339))
		fmt.Printf("    products %d, orders %d\n", t.products, t.orders)
		totalOrders += t.orders
		totalProducts += t.products
	}
	fmt.Printf("\nTotal: %d tenants, %d products, %d orders\n",
		len(targets), totalProducts, totalOrders)

	if !*confirm {
		fmt.Println("\nDry run. Re-run with --confirm to delete.")
		return
	}

	deleted, err := deleteTargets(ctx, db, targets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\ndelete: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nDeleted %d tenants and everything cascading from them.\n", deleted)
}

func findTargets(ctx context.Context, db *pgxpool.Pool, subdomain string, all bool) ([]tenant, error) {
	// The counts are reported before deleting so a dry run shows the
	// blast radius rather than just a tenant name.
	query := `
        SELECT t.id, t.subdomain, t.name, t.created_at,
               (SELECT COUNT(*) FROM orders   o WHERE o.tenant_id = t.id),
               (SELECT COUNT(*) FROM products p WHERE p.tenant_id = t.id)
        FROM tenants t
        WHERE t.subdomain = $1
        ORDER BY t.created_at
    `
	arg := subdomain

	if all {
		query = `
            SELECT t.id, t.subdomain, t.name, t.created_at,
                   (SELECT COUNT(*) FROM orders   o WHERE o.tenant_id = t.id),
                   (SELECT COUNT(*) FROM products p WHERE p.tenant_id = t.id)
            FROM tenants t
            WHERE t.subdomain LIKE $1
            ORDER BY t.created_at
        `
		arg = loadTestPrefix + "%"
	}

	rows, err := db.Query(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []tenant
	for rows.Next() {
		var t tenant
		if err := rows.Scan(&t.id, &t.subdomain, &t.name, &t.createdAt, &t.orders, &t.products); err != nil {
			return nil, err
		}
		// Belt and braces: the LIKE above should already guarantee this,
		// but a deletion loop is the wrong place to trust a query.
		if !strings.HasPrefix(t.subdomain, loadTestPrefix) {
			return nil, fmt.Errorf("refusing to continue: query returned non-load-test tenant %q", t.subdomain)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func deleteTargets(ctx context.Context, db *pgxpool.Pool, targets []tenant) (int64, error) {
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.id)
	}

	tag, err := db.Exec(ctx, `DELETE FROM tenants WHERE id = ANY($1)`, ids)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
