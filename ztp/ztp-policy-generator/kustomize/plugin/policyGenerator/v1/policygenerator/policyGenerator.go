package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	policyGen "github.com/openshift-kni/cnf-features-deploy/ztp/ztp-policy-generator/kustomize/plugin/policyGenerator/v1/policygenerator/policyGen"
	siteConfigs "github.com/openshift-kni/cnf-features-deploy/ztp/ztp-policy-generator/kustomize/plugin/policyGenerator/v1/policygenerator/siteConfig"
	utils "github.com/openshift-kni/cnf-features-deploy/ztp/ztp-policy-generator/kustomize/plugin/policyGenerator/v1/policygenerator/utils"
	"gopkg.in/yaml.v3"
)

var sourcePath string
var tempPath string
var outPath string
var stdout bool

func main() {

	tempPath = os.Args[2]
	sourcePath = os.Args[3]
	outPath = os.Args[4]
	stdout = (os.Args[5] == "true")
	InitiatePolicyGen(tempPath, sourcePath, outPath, stdout)
}

func InitiatePolicyGen(tempPath string, sourcePath string, outPath string, stdout bool) {

	fHandler := utils.NewFilesHandler(sourcePath, tempPath, outPath)

	files, err := fHandler.GetTempFiles()

	if err != nil {
		fmt.Println(err)
	}
	for _, file := range files {
		if file.Name()[0] == '.' {
			// Skip hidden files (for example, .gitignore or editor swap files)
			continue
		}

		kindType := utils.KindType{}
		yamlFile, err := fHandler.ReadTempFile(file.Name())

		if err != nil {
			fmt.Println(err)
		}

		err = yaml.Unmarshal(yamlFile, &kindType)
		if err != nil {
			fmt.Println(err)
		}

		if kindType.Kind == "PolicyGenTemplate" {
			policyGenTemp := utils.PolicyGenTemplate{}
			err := yaml.Unmarshal(yamlFile, &policyGenTemp)
			if err != nil {
				fmt.Println(err)
			}
			pBuilder := policyGen.NewPolicyBuilder(fHandler)
			policies, err := pBuilder.Build(policyGenTemp)

			if err != nil {
				fmt.Println(err)
				// The error will be raised after we write out whatever policy we can
			}
			for k, v := range policies {
				var policy []byte
				t := reflect.ValueOf(v)
				switch t.Kind() {
				case reflect.Slice:
					var buf bytes.Buffer
					for i := 0; i < t.Len(); i++ {
						b, err := yaml.Marshal(t.Index(i).Interface())
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error marshalling yaml for %s[%d]: %s\n", k, i, err)
						} else {
							if t.Len() > 0 {
								buf.WriteString("---\n")
							}
							buf.Write(b)
						}
					}
					policy = buf.Bytes()
				default:
					policy, err = yaml.Marshal(v)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error marshalling yaml for %s: %s", k, err)
					}
				}
				if stdout {
					strPolicy := string(policy)
					if !strings.HasPrefix(strPolicy, "---\n") {
						fmt.Println("---")
					}
					fmt.Println(strPolicy)
				}
				fHandler.WriteFile(k+utils.FileExt, policy)
			}
			if err != nil {
				panic(err)
			}
		} else if kindType.Kind == "SiteConfig" {
			siteConfig := siteConfigs.SiteConfig{}
			err := yaml.Unmarshal(yamlFile, &siteConfig)
			if err != nil {
				fmt.Println(err)
				panic(err)
			}
			var buffer bytes.Buffer
			scBuilder, err := siteConfigs.NewSiteConfigBuilder(fHandler)

			if err != nil {
				fmt.Println(err)
				panic(err)
			}
			clusters, err := scBuilder.Build(siteConfig)

			if err != nil {
				fmt.Println(err)
				// Error will be raised below after writing out whatever CR we can
			}
			for clusterName, crs := range clusters {
				for _, crIntf := range crs {
					cr, err := yaml.Marshal(crIntf)
					if err != nil {
						panic(err)
					}
					buffer.Write(siteConfigs.Separator)
					buffer.Write(cr)
				}

				if stdout {
					fmt.Println(buffer.String())
				}
				fHandler.WriteFile(clusterName+utils.FileExt, buffer.Bytes())
				buffer.Reset()
			}
			if err != nil {
				panic(err)
			}
		} else {
			log.Println("Not_supported_template ", kindType)
		}
	}
}
