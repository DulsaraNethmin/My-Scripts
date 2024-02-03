SHOW DATABASES;

CREATE database customer_care;

SELECT name FROM master.dbo.sysdatabases;


use customer_care;

CREATE TABLE Customers (
    CustomerID int IDENTITY(1,1) NOT NULL,
    FirstName varchar(255) NOT NULL,
    LastName varchar(255) NOT NULL,
    Email varchar(255) NOT NULL,
    PRIMARY KEY (CustomerID)
);

CREATE TABLE Orders (
    OrderID int IDENTITY(1,1) NOT NULL,
    OrderDate datetime NOT NULL,
    CustomerID int NOT NULL,
    PRIMARY KEY (OrderID),
    FOREIGN KEY (CustomerID) REFERENCES Customers(CustomerID)
);


SELECT name FROM master.dbo.sysdatabases

insert into Customers (FirstName, LastName, Email) values ('Alice', 'Brown', 
'alice@gmail.com');
insert into Customers (FirstName, LastName, Email) values ('Michael', 'Taylor',
'micheal@gmail.com');


select * from Customers;

EXEC sp_help 'Customers';



