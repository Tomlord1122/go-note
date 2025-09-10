package auth

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/supabase-community/supabase-go"
)

var (
	supabaseURL        = os.Getenv("SUPABASE_URL")
	supabaseKey        = os.Getenv("SUPABASE_ANON_KEY")
	supabaseServiceKey = os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
)

// SupabaseClient holds the Supabase client instance
type SupabaseClient struct {
	Client        *supabase.Client
	ServiceClient *supabase.Client
}

// NewSupabaseClient creates a new Supabase client
func NewSupabaseClient() (*SupabaseClient, error) {
	// Client for regular operations
	client, err := supabase.NewClient(supabaseURL, supabaseKey, &supabase.ClientOptions{})
	if err != nil {
		return nil, err
	}

	// Service client for admin operations
	serviceClient, err := supabase.NewClient(supabaseURL, supabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		return nil, err
	}

	return &SupabaseClient{
		Client:        client,
		ServiceClient: serviceClient,
	}, nil
}
