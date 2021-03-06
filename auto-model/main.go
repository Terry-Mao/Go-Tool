package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const (
	getColumnInfoSQL = `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_TYPE
    FROM COLUMNS WHERE TABLE_SCHEMA LIKE ? AND TABLE_NAME LIKE ?
    ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`
	modelHead   = "// auto-generate by Terry-Mao/auto-model\npackage model\nimport (\n${need_package})\ntype %s struct {\n"
	modelField  = "%s %s\n"
	modelTail   = "}"
	dbPackage   = "\"database/sql\"\n"
	timePackage = "\"time\"\n"
)

var (
	dbName   string
	tbName   string
	dbHost   string
	dbPort   int
	dbUser   string
	dbPwd    string
	dbSocket string
	dbDSN    string
	dir      string
	db       *sql.DB
	goRoot   string
)

func init() {
	flag.StringVar(&dbName, "d", "", "set the database name (use % for wild-char, etc DB% which means database name start with \"DB\")")
	flag.StringVar(&tbName, "t", "*", "set the table name (use % for wild-char, etc Table% which means table name start with \"Table\")")
	flag.StringVar(&dbHost, "h", "127.0.0.1", "set the database host ip")
	flag.StringVar(&dbUser, "u", "root", "set the database user")
	flag.StringVar(&dbPwd, "p", "", "set the database password")
	flag.StringVar(&dbSocket, "S", "", "set the socket file")
	flag.StringVar(&dir, "D", "./", "set the destination dir")
	flag.IntVar(&dbPort, "P", 3306, "set the database host port")

}

func main() {
	var (
		err         error
		preTable    = ""
		tableSchema string
		tableName   string
		columnName  string
		dataType    string
		isNullable  string
		columnType  string
		buf         = &bytes.Buffer{}
		packages    = map[string]bool{}
	)

	flag.Parse()

	if dbName == "" {
		fmt.Println("[ERROR] database name not set, please use -d=dbname")
		return
	}

	if dbSocket != "" {
		dbDSN = fmt.Sprintf("%s:%s@unix(%s)/INFORMATION_SCHEMA?charset=utf8", dbUser, dbPwd, dbSocket)
	} else {
		dbDSN = fmt.Sprintf("%s:%s@tcp(%s:%d)/INFORMATION_SCHEMA?charset=utf8", dbUser, dbPwd, dbHost, dbPort)
	}

	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	goRoot = runtime.GOROOT()
	if !strings.HasSuffix(goRoot, "/") {
		goRoot += "/"
	}

	// open db
	if db, err = sql.Open("mysql", dbDSN); err != nil {
		fmt.Println("Can't connect to mysql (%s)\n", err.Error())
		return
	}
	defer db.Close()

	// get column info
	stmt, err := db.Prepare(getColumnInfoSQL)
	if err != nil {
		fmt.Printf("Prepare sql failed (%s)\n", err.Error())
		return
	}
	defer stmt.Close()

	rows, err := stmt.Query(dbName, tbName)
	if err != nil {
		fmt.Printf("Query sql failed (%s)\n", err.Error())
		return
	}
	defer rows.Close()

	// enum rows
	for rows.Next() {
		if err = rows.Scan(&tableSchema, &tableName, &columnName, &dataType, &isNullable, &columnType); err != nil {
			fmt.Printf("Row scan failed (%s)\n", err.Error())
			return
		}

		if preTable != tableSchema+"."+tableName {
			// not same table
			if preTable != "" {
				// model end, write file
				flush(buf, packages, preTable[strings.LastIndex(preTable, ".")+1:])
			}

			// new model
			preTable = tableSchema + "." + tableName

			// model head
			if _, err = buf.WriteString(fmt.Sprintf(modelHead, firstUpper(tableName))); err != nil {
				fmt.Printf("Buffer WriteString failed (%s)", err.Error())
				return
			}
		}

		// model field
		packages[goPackage(dataType, isNullable)] = true
		if _, err = buf.WriteString(fmt.Sprintf(modelField, firstUpper(columnName), goType(dataType, isNullable, columnType))); err != nil {
			fmt.Printf("Buffer WriteString failed (%s)", err.Error())
			return
		}
	}

	// model end, write file
	flush(buf, packages, tableName)
}

func flush(buf *bytes.Buffer, packages map[string]bool, tableName string) {
	packageStr := ""

	// model end, write file
	if _, err := buf.WriteString(modelTail); err != nil {
		fmt.Printf("Buffer WriteString failed (%s)", err.Error())
		panic(err)
	}

	// replace packege
	for k, _ := range packages {
		delete(packages, k)
		if k == "" {
			continue
		}
		packageStr += k
	}

	// flush file
	fileName := dir + tableName + ".go"
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("OpenFile \"%s\" failed (%s)\n", fileName, err.Error())
		panic(err)
	}
	defer file.Close()

	file.WriteString(strings.Replace(buf.String(), "${need_package}", packageStr, 1))
	if err = file.Sync(); err != nil {
		fmt.Printf("File sync failed (%s)\n", err.Error())
		panic(err)
	}

	buf.Reset()
	goFmt(fileName)
}

func firstUpper(str string) string {
	return strings.ToUpper(str[0:1]) + str[1:]
}

func goPackage(str, null string) string {
	if null == "YES" {
		return dbPackage
	} else if str == "timestamp" {
		return timePackage
	}

	return ""
}

func goType(str, null, col string) string {
	isNull := (null == "YES")
	isUnsigned := (strings.Index(col, "unsigned") > 0)

	switch str {
	case "varchar", "char":
		if isNull {
			return "sql.NullString"
		} else {
			return "string"
		}
	case "binary":
		return "[]byte"
	case "timestamp", "date":
        if isNull {
            return "sql.NullString"
        } else {
		    return "string"
        }
	case "bit":
		if isNull {
			return "sql.NullBool"
		} else {
			return "Bool"
		}
	case "decimal":
		if isNull {
			return "sql.NullFloat64"
		} else {
			return "float64"
		}
	case "tinyint":
		if isNull {
			return "sql.NullInt64"
		} else {
			if isUnsigned {
				return "uint"
			} else {
				return "int"
			}
		}
	case "smallint":
		if isNull {
			return "sql.NullInt64"
		} else {
			if isUnsigned {
				return "uint"
			} else {
				return "int"
			}
		}
	case "int":
		if isNull {
			return "sql.NullInt64"
		} else {
			if isUnsigned {
				return "uint"
			} else {
				return "int"
			}
		}
	case "bigint":
		if isNull {
			return "sql.NullInt64"
		} else {
			if isUnsigned {
				return "uint64"
			} else {
				return "int64"
			}
		}
	case "tinyblob", "blob", "mediumblob", "longblob":
		return "[]byte"
	default:
		panic(fmt.Sprintf("unsupport database column type : %s, contact the lazy author\n", str))
	}
}

func goFmt(fileName string) {
	out := &bytes.Buffer{}

	cmd := exec.Command(goRoot+"bin/gofmt", "-w", fileName)
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sbin/gofmt -w %s*.go failed (%s)\n", goRoot, fileName, err.Error())
		fmt.Println(out.String())
		panic("goFmt")
	}
}
