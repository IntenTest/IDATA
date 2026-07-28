window.OhWembyTableDefinitions = Object.freeze({
  testCases: Object.freeze({
    endpoint: "/api/v1/test-cases",
    rowKey: "id",
    columns: Object.freeze([
      {
        field: "id",
        label: "ID",
        width: 90,
        renderer: "text",
        sortable: "custom",
      },
      {
        field: "title",
        label: "Title",
        minWidth: 300,
        renderer: "text",
        showOverflowTooltip: true,
      },
      { field: "owner", label: "Owner", width: 130, renderer: "text" },
      {
        field: "updated",
        label: "Updated",
        width: 120,
        renderer: "date",
        sortable: "custom",
      },
      {
        field: "actions",
        label: "Actions",
        width: 90,
        fixed: "right",
        renderer: "actions",
      },
    ]),
  }),
  testSuites: Object.freeze({
    endpoint: "/api/v1/test-suites",
    rowKey: "id",
    columns: Object.freeze([
      { field: "id", label: "ID", width: 100, renderer: "text" },
      {
        field: "name",
        label: "Name",
        minWidth: 210,
        renderer: "text",
        showOverflowTooltip: true,
      },
      {
        field: "description",
        label: "Description",
        minWidth: 280,
        renderer: "text",
        showOverflowTooltip: true,
      },
      {
        field: "caseIds",
        label: "Test cases",
        width: 100,
        align: "center",
        renderer: "count",
      },
      { field: "owner", label: "Owner", width: 130, renderer: "text" },
      { field: "updated", label: "Updated", width: 100, renderer: "text" },
      {
        field: "actions",
        label: "Actions",
        width: 150,
        fixed: "right",
        renderer: "actions",
      },
    ]),
  }),
});
