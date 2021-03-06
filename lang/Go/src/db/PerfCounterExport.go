////////////////////////////////////////////////////////////////////////////
// Porgram: PerfCounterExport
// Purpose: Export performance counters collected from MS load test to .csv
//			files for perfmon to view
// Authors: Tong Sun (c) 2013, All rights reserved
////////////////////////////////////////////////////////////////////////////

// Style: gofmt -tabs=false -tabwidth=4 -w

// Translated to GO from C#, http://blogs.msdn.com/b/geoffgr/archive/2013/09/09/

package main

import (
    "bitbucket.org/kardianos/table"
    _ "code.google.com/p/odbc"
    "database/sql"
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"
)

var progname string = "PerfCounterExport" // os.Args[0]

var (
    fSqlConnectionString = flag.String("c", "",
        "ConnectionString of Go MSSQL Odbc to MS SQL Server\n\tDefault: empty, which means using the -cs -cd setting. Sample: \n"+
            "  'driver=sql server;server=(local);database=LoadTest2010;uid=user;pwd=pass'\n")

    fServer = flag.String("cs", "(local)",
        "Connection Server, server to PerfCounter info from\n\tDefault: local machine.\n")

    fPerfDb = flag.String("cd", "LoadTest2010",
        "Connection DB, db that holds the PerfCounter info\n\tDefault: LoadTest2010\n")

    fLoadTestRunId = flag.Int("id", -1,
        "Loadtest RunId\n\tDefault: Max RunId\n")

    fMachineNameFilter = flag.String("m", "",
        "machineNameFilter for exporting the performance counters\n\tDefault: export all machines\n")

    fNoClobber = flag.Bool("nc", false, "no clobber, do not overwrite existing files\n\tDefault: overwrite them\n")

    fPathOnly = flag.Bool("po", false, "path only in ResultFilePre, no file prefix part.\n\tThe file prefix will therefore be the same as the folder name\n\tDefault: file prefix provided\n")

    fStep = flag.Int("s", 50,
        "Progress step\n\tDefault: Progress indicator every 50 loadtest record output\n")
)

func usage() {
    fmt.Fprintf(os.Stderr, "\nUsage:\n %s [flags] ResultFilePre\n\nFlags:\n\n",
        progname)
    flag.PrintDefaults()
    fmt.Fprintf(os.Stderr, "\nResultFilePre: \n\tThe prefix for the export files, including the path.\n\tThe machine names will be appended to it.\n\n\tE.g. C:\\Temp\\LoadTest-0822\n")
    os.Exit(0)
}

func main() {
    flag.Usage = usage
    flag.Parse()

    // The resultFilePre is a mandatory non-flag arguments
    if len(flag.Args()) < 1 {
        usage()
    }
    resultFilePre := flag.Args()[0]
    // if path only, append folder name as file prefix
    if *fPathOnly {
        resultFilePre += string(os.PathSeparator)
        resultFilePre = filepath.Dir(resultFilePre) +
            string(os.PathSeparator) + filepath.Base(resultFilePre)
        //log.Println("] resultFilePre=" + resultFilePre)
    }

    // Construct the Go MSSQL odbc SqlConnectionString
    // https://code.google.com/p/odbc/source/browse/mssql_test.go
    var c string
    if *fSqlConnectionString == "" {
        var params map[string]string
        params = map[string]string{
            "driver":             "sql server",
            "server":             *fServer,
            "database":           *fPerfDb,
            "trusted_connection": "yes",
        }

        for n, v := range params {
            c += n + "=" + v + ";"
        }
    } else {
        c = *fSqlConnectionString
    }
    log.Println("Connection string: " + c)

    conn, err := sql.Open("odbc", c)
    if err != nil {
        fmt.Println("Connecting Error")
        return
    }
    defer conn.Close()

    log.Printf("[%s] Program started\n", progname)

    if *fLoadTestRunId < 0 {
        // No Loadtest specified. Use Max RunId.
        runId, err := table.Get(conn,
            "SELECT MAX(LoadTestRunId) AS RunId from LoadTestRun")
        if err != nil {
            log.Fatal(err)
        }

        maxRunId := runId.MustGetScaler(0, "RunId")
        *fLoadTestRunId = int(maxRunId.(int32))
    }
    log.Printf("[%s] Exporting LoadTest %d\n  to %s-...\n  with progress step of %d\n",
        progname, *fLoadTestRunId, resultFilePre, *fStep)

    if *fMachineNameFilter != "" {
        fmt.Printf("  limiting to only export machine %s\n\n", *fMachineNameFilter)
        savePerfmonAsCsv(fNoClobber, conn, *fMachineNameFilter, *fLoadTestRunId, resultFilePre)
        os.Exit(0)
    }

    /*
    		Get all machine names

    	    SELECT  category.MachineName
    	      FROM  LoadTestPerformanceCounterCategory AS category
    	      JOIN  LoadTestPerformanceCounterInstance AS instance
    	        ON  category.LoadTestRunId = instance.LoadTestRunId
    	       AND  instance.LoadTestRunId = (
    	            SELECT MAX(LoadTestRunId) from LoadTestRun )
    	     GROUP  BY MachineName

    */

    machines, err := table.Get(conn,
        "SELECT  category.MachineName"+
            "  FROM  LoadTestPerformanceCounterCategory AS category"+
            "  JOIN  LoadTestPerformanceCounterInstance AS instance"+
            "    ON  category.LoadTestRunId = instance.LoadTestRunId"+
            "   AND  instance.LoadTestRunId = ?"+
            " GROUP  BY MachineName", *fLoadTestRunId)
    if err != nil {
        log.Fatal(err)
    }

    for _, machine := range machines.Rows {
        machineName := machine.MustGet("MachineName").(string)
        savePerfmonAsCsv(fNoClobber, conn, machineName, *fLoadTestRunId, resultFilePre)
    }

    log.Printf("[%s] Exporting finished correctly.\n", progname)
    return
}

func savePerfmonAsCsv(fNoClobber *bool, conn *sql.DB, machineName string, _runId int, resultFilePre string) {
    // Only use right(5) 
    const keep = 5
    if len(machineName) > keep {
        machineName = machineName[len(machineName)-keep:]
    }

    log.Printf("[%s]   Collecting data for %s...\n", progname, machineName)

    // if no clobber and the destination file exists, skip
    if *fNoClobber {
        if _, err := os.Stat(resultFilePre + "-" + machineName + ".csv"); err == nil {
            log.Printf("[%s]   (Host %s skipped for no clobbering)\n",
                progname, machineName)
            return
        }
    }

    sql := fmt.Sprintf("exec TSL_prc_PerfCounterCollectionInCsvFormat"+
        " @RunId = %d, @InstanceName=N'\\\\%%%s\\%%'", _runId, machineName)
    //log.Println("] sql string: " + sql)
    table, err := table.Get(conn, sql)
    if err != nil {
        log.Printf("[%s]   Skipping it for the fatal error:\n\t\t    %v\n",
            progname, err.Error())
        return
    }

    log.Printf("[%s]   Exporting %s data...\n", progname, machineName)

    // open the output file
    file, err := os.Create(resultFilePre + "-" + machineName + ".csv")
    if err != nil {
        panic(err)
    }
    // close file on exit and check for its returned error
    defer func() {
        if err := file.Close(); err != nil {
            panic(err)
        }
    }()

    // output header
    for i, element := range table.ColumnName {
        if i != 0 {
            fmt.Fprintf(file, ",")
        }
        fmt.Fprintf(file, "\"%s\"", element)
    }
    fmt.Fprintf(file, "\n")

    // output body
    const layout = "01/02/2006 15:04:05.999"
    for j, row := range table.Rows {
        for i, colname := range table.ColumnName {
            if i != 0 {
                fmt.Fprintf(file, ",")
            }
            switch x := row.MustGet(colname).(type) {
            case string: // x is a string
                fmt.Fprintf(file, "\"%s\"", x)
            case int: // now x is an int
                fmt.Fprintf(file, "\"%d\"", x)
            case int32: // now x is an int32
                fmt.Fprintf(file, "\"%d\"", x)
            case int64: // now x is an int64
                fmt.Fprintf(file, "\"%d\"", x)
            case float32: // now x is an float32
                fmt.Fprintf(file, "\"%f\"", x)
            case float64: // now x is an float64
                fmt.Fprintf(file, "\"%f\"", x)
            case time.Time: // now x is a time.Time
                fmt.Fprintf(file, "\"%s\"", x.Format(layout))
            default:
                fmt.Fprintf(file, "\"%s\"", x)
            }
        }
        fmt.Fprintf(file, "\n")
        if j%*fStep == 0 {
            fmt.Fprintf(os.Stderr, ".")
        }
    }
    fmt.Fprintf(os.Stderr, "\n")

}
