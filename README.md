
![image](https://github.com/user-attachments/assets/40b7bc7e-872c-4ce9-b13e-d6cd5829f87d)


This script crawls through a domain to find search terms.

Still fairly basic.

Define domains and search terms in the .yaml file (note that subdomains must be defined as their own domains).
Run using ./src 

- External links are not parsed
- Subdomains not parsed unless defined as their own domains
- Script only parses CSS, JS, and HTML files linked within the same domain
- Script does not download any files locally
- Script does not respect robots.txt
- Script ignores mailto, fax, sms, geo, and skype hrefs
- Script currently excludes relative links containing ".."
- Script does not space out requests by default, define seconds to wait between requests using -w
- Use output pipe to save output to file (example: ./src > output.txt)
