show databases;

create database school_managment_system;

use school_managment_system;

create table Students(
	index_number varchar(6),
    name varchar(40),
    email varchar(40),
    garde integer
);

select * from Students;

show create table Students;