Feature: The page
  As a visitor
  I want the page to greet me
  So that I know it has loaded

  @ID-APP-01 @phase-0
  Scenario: The page opens and greets the visitor by name
    Given the page is open for "Ada"
    Then the heading reads "Hello, Ada!"
    And no page errors were reported
