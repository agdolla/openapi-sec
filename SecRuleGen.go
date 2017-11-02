package main

import (
	"fmt"
	"net/http"
	"os"
	"io/ioutil"
	"encoding/json"
    "strings"
    "strconv"
    //"bufio"
	)

var modsecRuleID int = 30000

type Config struct {
    Url string `json:"url"`
    Ratelimit int `json:"ratelimit"`
    RatelimitWhitelist string `json:"ratelimit_whitelist"`
    RestrictedEndpoints []RestrictedEndpoint `json:"restricted_endpoints"`
    WebServer string `json:"webserver"`
    Verbose bool `json:"verbose"`
}
 
type RestrictedEndpoint struct {
    Path string `json:"path"`
    IpAllowed string `json:"ip_allowed"`
}

type Endpoint struct {
    Url string
    Methods []Method
}

type Method struct {
	Name string
	Parameters []EndpointParameter
}

type EndpointParameter struct {
    Name string `json:"name"`
    In string `json:"in"`
    Description string `json:"description"`
    Format string `json:"format"`
    Type string `json:"type"`
    Required bool `json:"required"`
    Enum []string `json:"enum"`
    Schema ParameterSchema `json:"schema"`
}

type ParameterSchema struct {
	Ref string `json:"$ref"`
	Type string `json:"type"`
}

type Definition struct {
	Name string
	Type string `json:"type"`
	Properties []PropertyDefinition `json:"properties"`
}

type PropertyDefinition struct {
	Name string
	Type string `json:"type"`
	Format string `json:"format"`
	Ref string `json:"$ref"`
}

func ReadConfigFile(s string) Config {
    configFile, err := os.Open(s)
    defer configFile.Close()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Err : unable to read %s\n", s)
        os.Exit(1)
    }
    bytes, _ := ioutil.ReadAll(configFile)
    var config Config
    json.Unmarshal(bytes, &config)
    return config
}

func GetSwaggerSpec(s string) interface{} {
    response, err := http.Get(s)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Err : unable to download %s\n", s)
        os.Exit(1)
    }
    body, err := ioutil.ReadAll(response.Body)
    var swaggerSpec interface{}
    json.Unmarshal(body, &swaggerSpec)
    return swaggerSpec
}

func GetEndpointList(s map[string]interface{}) []Endpoint {
    var list []Endpoint
    for endpoint, availableMethods := range s["paths"].(map[string]interface{}) {
    	var methods []Method
    	var parameters []EndpointParameter
    	for method, description := range availableMethods.(map[string]interface{}){
    		for property, v := range description.(map[string]interface{}){
    			if property == "parameters" {
    				switch x := v.(type) {
						case []interface{}:
    						for _, e := range x {
        						var p EndpointParameter
        						s, _ := json.Marshal(e)
        						json.Unmarshal(s, &p)
        						if len(p.Schema.Ref) > 0 {
        							for _, v := range _ParseParameterDefinition(p.Name, p.Schema.Ref) {
        								parameters = append(parameters, v)
        							}
        						} else {
        							p.Type = "string"
        							parameters = append(parameters, p)
        						}
    						}
						default:
    						fmt.Printf("can't parse parameter %T\n", v)
					}
    			}
    		}
    		methods = append(methods, Method{method, parameters})
    	}
    	list = append(list, Endpoint{endpoint, methods})
    }
    return list
}

func GetDefinitionList(s map[string]interface{}) []Definition {
	var definitions []Definition
	for name, properties := range s["definitions"].(map[string]interface{}) {
		var d Definition
		m := properties.(map[string]interface{})
		d.Name = "#/definitions/" + name
		d.Type = m["type"].(string)
		if d.Type == "object" {
			var p PropertyDefinition
			s, _ := json.Marshal(m["properties"])
			json.Unmarshal(s, &p)
			fmt.Println(m["properties"], " -> ", p)
			//for _, property := range m["properties"].(map[string]interface{}) {
			//	d.Properties = append (d.Properties, PropertyDefinition{ "property" })
			//}
		}
		definitions = append(definitions, d)
	}
	fmt.Println(definitions)
	return definitions
}

func GenerateRules(e []Endpoint, c Config ) []string {
    var rules []string
    var outputStyle string
    //var ratelimit string = c.Ratelimit
    if c.WebServer == "apache" || c.WebServer == "nginx" {
        outputStyle = c.WebServer
    } else {
        return rules
    }
    for _, endpoint := range e {
    	rules = append(rules, _GenerateLocationBlockHeader(endpoint, outputStyle))
    	if len(_GenerateSourceIpAddrRules(endpoint, c)) > 0 {
    		for _, r := range _GenerateSourceIpAddrRules(endpoint, c) {
    			rules = append(rules, r)
    		}
    	}
    	rules = append(rules, _GenerateMethodRule(endpoint))
    	if len(_GenerateRatelimitRules(endpoint, c)) > 0{
    		for _, line := range _GenerateRatelimitRules(endpoint, c) {
    			rules = append(rules, line)
    		}
    	}
    	for _, line := range _GeneratePerMethodRules(endpoint){
    		rules = append(rules, line)
    	}
    	rules = append(rules, _GenerateLocationBlockFooter(outputStyle))
    }
    return rules
}

func _ParseParameterDefinition(n string, r string) []EndpointParameter {
	var parameters []EndpointParameter

	return parameters
}

func _GeneratePerMethodRules(e Endpoint) []string {
	var rules []string
	for _, method := range e.Methods {
		id := strconv.Itoa(_GetModsecRuleID())
		rules = append(rules, "SecRule REQUEST_METHOD \"!^(?:"+ strings.ToUpper(method.Name) + ")$\" \"skipAfter:FILTER_BY_METHOD_" + id + ",nolog,id:'" + id + "'\"")

		rules = append(rules, "SecMarker FILTER_BY_METHOD_" + id)
	}
	return rules
}

func _GenerateRatelimitRules(e Endpoint, c Config) []string {
	var rules []string
	if len(c.RatelimitWhitelist) == 0 || c.Ratelimit == 0 {
		return rules
	}
	id := strconv.Itoa(_GetModsecRuleID())
	rules = append(rules, "SecRule REMOTE_ADDR \"@ipMatch " + c.RatelimitWhitelist + "\" \"skipAfter:IGNORE_RATELIMIT_" + id + ",nolog,id:'" + id + "'\"")
	rules = append(rules, "SecAction \"initcol:ip=%{REMOTE_ADDR}_%{REQUEST_HEADERS.User-Agent},pass,nolog,id:'" + strconv.Itoa(_GetModsecRuleID()) + "'\"")
	rules = append(rules, "SecAction \"phase:5,deprecatevar:ip./v1/professionnels/chantiers/count/maj-crm=100/60,pass,nolog,id:'" + strconv.Itoa(_GetModsecRuleID()) + "'\"")
	rules = append(rules, "SecAction \"phase:2,pass,setvar:ip./v1/professionnels/chantiers/count/maj-crm=+1,nolog,id:'" + strconv.Itoa(_GetModsecRuleID()) + "'\"")
	rules = append(rules, "SecRule IP:/v1/professionnels/chantiers/count/maj-crm \"@gt " + strconv.Itoa(c.Ratelimit) + "\" \"phase:2,pause:300,deny,setenv:RATELIMITED,skip:1,id:'" + strconv.Itoa(_GetModsecRuleID()) + "',status:400,msg:'too many request per minute',logdata:%{MATCHED_VAR}\"")
	rules = append(rules, "SecMarker IGNORE_RATELIMIT_" + id)
	return rules
}

func _GenerateMethodRule(e Endpoint) string{
	rule := "SecRule REQUEST_METHOD \"!^(?:"
	var methods string
	for _ , method := range e.Methods {
		if len(methods) == 0 {
			methods = methods + strings.ToUpper(method.Name)
		} else {
			methods = methods + "|" + strings.ToUpper(method.Name)
		}
	}
	methods = methods + "|OPTIONS"
	rule = rule + methods
	rule = rule + ")$\" \"phase:2,t:none,deny,id:'" + strconv.Itoa(_GetModsecRuleID()) + "',status:405,msg:'Unauthorize method',logdata:%{REQUEST_METHOD},setenv:METHODERROR\""
	return rule
}

func _GenerateSourceIpAddrRules(e Endpoint, c Config ) []string {
	var rules []string
	if len(c.RestrictedEndpoints) > 0{
		for _, v := range c.RestrictedEndpoints {
			if v.Path == e.Url {
				id1 := strconv.Itoa(_GetModsecRuleID())
				rules = append(rules, "SecRule REMOTE_ADDR \"@ipMatch " + v.IpAllowed + "\" \"id:'" + id1 + "',skipAfter:IP_IS_ALLOWED_" + id1 + ",nolog\"")
				rules = append(rules, "SecAction \"deny,id:'" + strconv.Itoa(_GetModsecRuleID()) + "',log,msg:'IP not allowed on this endpoint',logdata:%{MATCHED_VAR}\"")
				rules = append(rules, "SecMarker IP_IS_ALLOWED_" + id1)
			}
		}
	}
	return rules
}

func _GenerateLocationBlockHeader(e Endpoint, s string) string{
    var locationBlockHeader string
	fmt.Println(e.Url)
    url := _URLParameterToRegex(e)
	fmt.Println("\t->", url)
    if s == "apache" {
        locationBlockHeader = "<LocationMatch \"^"+ url + "$\">"
    }
    if s == "nginx" {
    	locationBlockHeader = "location " + url + "{ modsecurity_rules '"
    }
    return locationBlockHeader
}

func _GenerateLocationBlockFooter(s string) string {
	var block string
	if s == "apache" {
		block = "</LocationMatch>"
	}
	if s == "nginx" {
		block =   "';}"
	}
	return block
}

func _URLParameterToRegex(e Endpoint) string {
	buff := e
	if strings.ContainsAny("{", buff.Url) == false {
		return buff.Url
	}
	urlParameter := buff.Url[(strings.Index(buff.Url, "{"))+1:strings.Index(buff.Url, "}")]
    for _, method := range buff.Methods {
        for _, parameter := range method.Parameters {
            if parameter.Name == urlParameter {
				fmt.Println(urlParameter)
            	fmt.Println(strings.Index(buff.Url, urlParameter))
            	buff.Url = e.Url[:strings.Index(buff.Url, urlParameter)-1] + "("
				buff.Url = buff.Url + _TypeToRegex(parameter.Type) + ")"
				fmt.Println(e.Url[strings.Index(buff.Url, urlParameter):])
				buff.Url = buff.Url + e.Url[strings.Index(buff.Url, urlParameter):]
            }
        }
    }
	if strings.ContainsAny("{", buff.Url) == true {
		return _URLParameterToRegex(buff)
	}
    return buff.Url
}

func _TypeToRegex(t string) string{
    switch t {
    case "boolean":
        return "[0-1]"
    case "integer":
        return "[+-]?[0-9]*"
    case "number":
        return "[+-]?[0-9\\.,]*"
    case "string":
        return "[^/]*"
    default:
        return ""
    }
}

func _GetModsecRuleID() int {
	modsecRuleID = modsecRuleID + 1
	return modsecRuleID
}

func main() {
	config := ReadConfigFile("C:\\Users\\joanelis\\Documents\\openapi-sec\\test3.json")
//    if len(os.Args) != 2 {
//        fmt.Fprintf(os.Stderr, "Usage: %s config.json\n", os.Args[0])
//        os.Exit(1)
//    }
//    config := readConfigFile(os.Args[1])
    swaggerSpecs := GetSwaggerSpec(config.Url).(map[string]interface{})
    fmt.Println("Version of swagger specifications : ", swaggerSpecs["swagger"], "\nParsing its content...\n")
    endpoints := GetEndpointList(swaggerSpecs)
    definitions := GetDefinitionList(swaggerSpecs)
    rules := GenerateRules(endpoints, config)
    //GetDefinitionList(swaggerSpecs)
    for _, v := range rules {
    	fmt.Println(v)
    }
    fmt.Println(definitions)
}