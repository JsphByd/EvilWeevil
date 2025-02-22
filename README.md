This script crawls through a domain to find search terms. 
Define domains and search terms in the .yaml file (note that subdomains must be defined as their own domains).
Run using ./src 

- External links are not parsed
- Subdomains not parsed unless defined as their own domains
- Script only parses CSS, JS, and HTML files linked within the same domain
- Script does not download any files locally
- Script does not respect robots.txt
- Script does not space out requests by default, define seconds to wait between requests using -w 
