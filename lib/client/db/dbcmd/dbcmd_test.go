package dbcmd

import (
	"errors"
	"testing"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// fakeExec implements execer interface for mocking purposes.
type fakeExec struct {
	// execOutput maps binary name and output that should be returned on RunCommand().
	// Map is also being used to check if a binary exist. Command line args are not supported.
	execOutput map[string][]byte
}

func (f fakeExec) RunCommand(cmd string, _ ...string) ([]byte, error) {
	out, found := f.execOutput[cmd]
	if !found {
		return nil, errors.New("binary not found")
	}

	return out, nil
}

func (f fakeExec) LookPath(path string) (string, error) {
	if _, found := f.execOutput[path]; found {
		return "", nil
	}
	return "", trace.NotFound("not found")
}

func TestCliCommandBuilderGetConnectCommand(t *testing.T) {
	conf := &client.Config{
		HomePath:     t.TempDir(),
		Host:         "localhost",
		WebProxyAddr: "localhost",
		SiteName:     "db.example.com",
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
	}

	tests := []struct {
		name         string
		dbProtocol   string
		databaseName string
		execer       *fakeExec
		cmd          []string
		noTLS        bool
		wantErr      bool
	}{
		{
			name:         "postgres",
			dbProtocol:   defaults.ProtocolPostgres,
			databaseName: "mydb",
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:         "postgres no TLS",
			dbProtocol:   defaults.ProtocolPostgres,
			databaseName: "mydb",
			noTLS:        true,
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb"},
			wantErr: false,
		},
		{
			name:         "cockroach",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"cockroach": []byte(""),
				},
			},
			cmd: []string{"cockroach", "sql", "--url",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:         "cockroach no TLS",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			noTLS:        true,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"cockroach": []byte(""),
				},
			},
			cmd: []string{"cockroach", "sql", "--url",
				"postgres://myUser@localhost:12345/mydb"},
			wantErr: false,
		},
		{
			name:         "cockroach psql fallback",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			execer:       &fakeExec{},
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:         "mariadb",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mariadb": []byte(""),
				},
			},
			cmd: []string{"mariadb",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP",
				"--ssl-key", "/tmp/keys/example.com/bob",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:         "mariadb no TLS",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			noTLS:        true,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mariadb": []byte(""),
				},
			},
			cmd: []string{"mariadb",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "mysql by mariadb",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("mysql  Ver 15.1 Distrib 10.3.32-MariaDB, for debian-linux-gnu (x86_64) using readline 5.2"),
				},
			},
			cmd: []string{"mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP",
				"--ssl-key", "/tmp/keys/example.com/bob",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:         "mysql by oracle",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("Ver 8.0.27-0ubuntu0.20.04.1 for Linux on x86_64 ((Ubuntu))"),
				},
			},
			cmd: []string{"mysql",
				"--defaults-group-suffix=_db.example.com-mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "mysql no TLS",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			noTLS:        true,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("Ver 8.0.27-0ubuntu0.20.04.1 for Linux on x86_64 ((Ubuntu))"),
				},
			},
			cmd: []string{"mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "no mysql nor mariadb returns default mysql command",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mysql",
				"--defaults-group-suffix=_db.example.com-mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "mongodb (legacy)",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mongo",
				"--host", "localhost",
				"--port", "12345",
				"--ssl",
				"--sslPEMKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"mydb"},
			wantErr: false,
		},
		{
			name:         "mongodb no TLS",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			noTLS:        true,
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mongo",
				"--host", "localhost",
				"--port", "12345",
				"mydb"},
			wantErr: false,
		},
		{
			name:         "mongosh",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--host", "localhost",
				"--port", "12345",
				"--tls",
				"--tlsCertificateKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--tlsUseSystemCA",
				"mydb"},
		},
		{
			name:         "mongosh no TLS",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			noTLS:        true,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--host", "localhost",
				"--port", "12345",
				"mydb"},
		},
		{
			name:         "sqlserver",
			dbProtocol:   defaults.ProtocolSQLServer,
			databaseName: "mydb",
			cmd: []string{mssqlBin,
				"-S", "localhost,12345",
				"-U", "myUser",
				"-P", fixtures.UUID,
				"-d", "mydb",
			},
			wantErr: false,
		},
		{
			name:       "redis-cli",
			dbProtocol: defaults.ProtocolRedis,
			cmd: []string{"redis-cli",
				"-h", "localhost",
				"-p", "12345",
				"--tls",
				"--key", "/tmp/keys/example.com/bob",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem"},
			wantErr: false,
		},
		{
			name:         "redis-cli with db",
			dbProtocol:   defaults.ProtocolRedis,
			databaseName: "2",
			cmd: []string{"redis-cli",
				"-h", "localhost",
				"-p", "12345",
				"--tls",
				"--key", "/tmp/keys/example.com/bob",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"-n", "2"},
			wantErr: false,
		},
		{
			name:       "redis-cli no TLS",
			dbProtocol: defaults.ProtocolRedis,
			noTLS:      true,
			cmd: []string{"redis-cli",
				"-h", "localhost",
				"-p", "12345"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			database := &tlsca.RouteToDatabase{
				Protocol:    tt.dbProtocol,
				Database:    tt.databaseName,
				Username:    "myUser",
				ServiceName: "mysql",
			}

			opts := []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
			}
			if tt.noTLS {
				opts = append(opts, WithNoTLS())
			}

			c := NewCmdBuilder(tc, profile, database, "root", opts...)
			c.uid = utils.NewFakeUID()
			c.exe = tt.execer
			got, err := c.GetConnectCommand()
			if tt.wantErr {
				if err == nil {
					t.Errorf("getConnectCommand() should return an error, but it didn't")
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.cmd, got.Args)
		})
	}
}
