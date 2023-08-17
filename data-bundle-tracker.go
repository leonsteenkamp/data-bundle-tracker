package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	g "github.com/gosnmp/gosnmp"
	"log"
	"math/big"
	"os"
	"time"
	// "path/filepath"
	// "bytes"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type Options struct {
	flagOidIn         *string
	flagOidOut        *string
	flagAgentAddress  *string
	flagSnmpCommunity *string
	flagSnmpVersion   *int
	flagDebug         *bool
	flagLoadVoucher   *string
}

var opts Options

func init() {
	// OID IF-MIB::ifInOctets.2
	opts.flagOidIn = flag.String("in", "1.3.6.1.2.1.2.2.1.10.2", "OID string")
	// OID IF-MIB::ifOutOctets.2
	opts.flagOidOut = flag.String("out", "1.3.6.1.2.1.2.2.1.16.2", "OID string")
	// Agent address
	opts.flagAgentAddress = flag.String("agent", "192.168.1.1", "Agent IP address")
	// Community string
	opts.flagSnmpCommunity = flag.String("community", "public", "SNMP community string")
	// SNMP version
	opts.flagSnmpVersion = flag.Int("version", 1, "SNMP version")
	// Print debug ouput
	opts.flagDebug = flag.Bool("debug", false, "Print debug output")
	// Load voucher
	opts.flagLoadVoucher = flag.String("voucher", "0", "Voucher amount to load in GB")

	flag.Parse()

	if *opts.flagDebug {
		fmt.Println("flagOidIn - ", *opts.flagOidIn)
		fmt.Println("flagOidOut - ", *opts.flagOidOut)
		fmt.Println("flagAgentAddress - ", *opts.flagAgentAddress)
		fmt.Println("flagSnmpCommunity - ", *opts.flagSnmpCommunity)
		fmt.Println("flagSnmpVersion - ", *opts.flagSnmpVersion)
		fmt.Println("flagLoadVoucher - ", *opts.flagLoadVoucher)
		fmt.Println("Extra:", flag.Args())
	}

}

func getOctets() (*big.Int, *big.Int) {
	g.Default.Target = *opts.flagAgentAddress

	err := g.Default.Connect()
	if err != nil {
		log.Fatalf("Connect() err: %v", err)
	}
	defer g.Default.Conn.Close()

	oids := []string{*opts.flagOidIn, *opts.flagOidOut}
	result, err2 := g.Default.Get(oids) // Get() accepts up to g.MAX_OIDS
	if err2 != nil {
		log.Fatalf("Get() err: %v", err2)
	}

	snmpValueIn := big.NewInt(0)
	snmpValueOut := big.NewInt(0)
	oidName := ""

	for i, variable := range result.Variables {
		// the Value of each variable returned by Get() implements
		// interface{}. You could do a type switch...
		switch variable.Type {
		case g.OctetString:
			bytes := variable.Value.([]byte)
			log.Printf("string: %s\n", string(bytes))
		default:
			// ... or often you're just interested in numeric values.
			// ToBigInt() will return the Value as a BigInt, for plugging
			// into your calculations.

			if variable.Name[1:] == *opts.flagOidIn {
				oidName = "ifInOctets "
				snmpValueIn = g.ToBigInt(variable.Value)
			}
			if variable.Name[1:] == *opts.flagOidOut {
				oidName = "ifOutOctets"
				snmpValueOut = g.ToBigInt(variable.Value)
			}

			value := g.ToBigInt(variable.Value)
			ans := big.NewInt(0)
			ans1 := big.NewInt(0)

			log.Printf("%d %s %s - %d B (%d KB) (%d MB)\n", i, oidName,
				variable.Name,
				value,
				ans.Div(value, big.NewInt(1000)),
				ans1.Div(ans, big.NewInt(1000)))
		}
	}

	return snmpValueIn, snmpValueOut
}

func readFile(filename string) (*big.Int, *big.Int, bool) {
	readDownNumber := big.NewInt(0)
	readUpNumber := big.NewInt(0)
	// var okd bool
	// var oku bool
	readFileOk := true

	pwd, err := os.Getwd()
	check(err)

	readFilePath := pwd + "/" + filename
	// fmt.Printf("Reading file: %s\n", readFilePath)

	if _, err := os.Stat(readFilePath); err == nil {
		// fmt.Printf("Opening existing file\n")
		f, err := os.Open(readFilePath)
		check(err)
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Scan()
		check(scanner.Err())
		// readDownNumber, okd = new(big.Int).SetString(scanner.Text(), 0)
		readDownNumber, _ = new(big.Int).SetString(scanner.Text(), 0)
		// fmt.Printf("Bytes down %d, %t\n", readDownNumber, okd)

		scanner.Scan()
		check(scanner.Err())
		// readUpNumber, oku = new(big.Int).SetString(scanner.Text(), 0)
		readUpNumber, _ = new(big.Int).SetString(scanner.Text(), 0)
		// fmt.Printf("Bytes up   %d, %t\n", readUpNumber, oku)

	} else if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("File did not exist.\n")
		readFileOk = false
	} else {
		fmt.Printf("Some other issue occurred\n")
		readFileOk = false
		panic("Some file issue, check permissions maybe.")
	}

	return readDownNumber, readUpNumber, readFileOk
}

func writeFile(filename string, downCount *big.Int, upCount *big.Int) bool {
	writeOk := true

	f1, err1 := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0600)
	check(err1)
	defer f1.Close()
	_, err2 := f1.WriteString(downCount.String() + "\n")
	check(err2)
	_, err2 = f1.WriteString(upCount.String())
	check(err2)

	return writeOk
}

func processCount(snmpValueInOld *big.Int, snmpValueIn *big.Int,
	snmpValueOutOld *big.Int, snmpValueOut *big.Int) (*big.Int, *big.Int) {
	difUp := big.NewInt(0)
	difUp1 := big.NewInt(0)
	difDown := big.NewInt(0)
	difDown1 := big.NewInt(0)
	totalUp := big.NewInt(0)
	totalDown := big.NewInt(0)
	data_cnt_file := "data_cnt.txt"

	downDataCnt, upDataCnt, _ := readFile(data_cnt_file)

	// Could add support for counting roll over value if a rollover happened

	if snmpValueIn.Cmp(snmpValueInOld) > 0 {
		difDown.Sub(snmpValueIn, snmpValueInOld)
	} else {
		fmt.Printf("Warning: new value smaller than old value\n")
	}
	if snmpValueOut.Cmp(snmpValueOutOld) > 0 {
		difUp.Sub(snmpValueOut, snmpValueOutOld)
	} else {
		fmt.Printf("Warning: new value smaller than old value\n")
	}

	totalDown.Add(difDown, downDataCnt)
	totalUp.Add(difUp, upDataCnt)
	writeFile(data_cnt_file, totalDown, totalUp)

	log.Printf("Interval difference %d kB %d kB\n", difDown1.Div(difDown, big.NewInt(1000)),
		difUp1.Div(difUp, big.NewInt(1000)))
	log.Printf("Counts (down/up) %d MB %d MB\n", totalDown.Div(totalDown, big.NewInt(1000000)),
		totalUp.Div(totalUp, big.NewInt(1000000)))

	return difDown, difUp
}

func loadVoucher(valueString string) {
	log.Printf("Loading voucher of %s GB", *opts.flagLoadVoucher)
	voucherFile := "voucher.txt"
	valueBigInt, _ := new(big.Int).SetString(valueString, 0)
	valueBigInt.Mul(valueBigInt, big.NewInt(1000000000))
	voucherValueDown, voucherValueUp, _ := readFile(voucherFile)
	voucherValueDown.Add(voucherValueDown, valueBigInt)
	voucherValueUp.Add(voucherValueUp, big.NewInt(0))
	writeFile(voucherFile, voucherValueDown, voucherValueUp)
}

func updateVoucher(difDown *big.Int, difUp *big.Int) {
	voucherFile := "voucher.txt"
	voucherValueDown, _, _ := readFile(voucherFile)
	voucherValueDown.Sub(voucherValueDown, difDown)
	voucherValueDown.Sub(voucherValueDown, difUp)
	writeFile(voucherFile, voucherValueDown, big.NewInt(0))

	log.Printf("Voucher value remaining: %d MB\n",
		voucherValueDown.Div(voucherValueDown, big.NewInt(1000000)))
}

func main() {
	if *opts.flagLoadVoucher != "0" {
		loadVoucher(*opts.flagLoadVoucher)
		os.Exit(0)
	}

	duration := time.Duration(10) * time.Second
	snmpValueIn, snmpValueOut := getOctets()

	for {
		time.Sleep(duration)
		snmpValueInOld := snmpValueIn
		snmpValueOutOld := snmpValueOut

		snmpValueIn, snmpValueOut = getOctets()
		difDown, difUp := processCount(snmpValueInOld, snmpValueIn,
			snmpValueOutOld, snmpValueOut)
		updateVoucher(difDown, difUp)

	}
}
